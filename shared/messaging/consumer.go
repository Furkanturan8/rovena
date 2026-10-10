package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	headerRetryCount = "x-retry-count"
	headerError      = "x-error"
	headerFailedAt   = "x-failed-at"

	reconnectDelay = 2 * time.Second
)

// Handler processes one event. Returning nil acknowledges it. Returning an error sends it
// to the retry queue until retries run out, then to the DLQ; wrap the error in Permanent
// when retrying cannot help. Handlers see the same event more than once, so they must be
// idempotent (see outbox.Inbox).
type Handler func(ctx context.Context, env Envelope) error

type permanentError struct{ err error }

func (p permanentError) Error() string { return p.err.Error() }
func (p permanentError) Unwrap() error { return p.err }

// Permanent marks err as not worth retrying (an invalid payload, a business rule that
// will never pass): the message goes straight to the DLQ.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// Consume handles messages from sub's queue until ctx is cancelled. A lost connection is
// reopened after a short delay; messages that were being processed are redelivered by
// the broker. It returns an error only for an invalid subscription.
func (b *Broker) Consume(ctx context.Context, sub Subscription, h Handler) error {
	if err := sub.validate(); err != nil {
		return err
	}
	for ctx.Err() == nil {
		err := b.consumeOnce(ctx, sub, h)
		if ctx.Err() != nil {
			return nil
		}
		b.log.ErrorContext(ctx, "consumer stopped, reconnecting", "queue", sub.Queue, "error", err)
		select {
		case <-ctx.Done():
		case <-time.After(reconnectDelay):
		}
	}
	return nil
}

func (b *Broker) consumeOnce(ctx context.Context, sub Subscription, h Handler) error {
	conn, err := b.connection(ctx)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open consumer channel: %w", err)
	}
	defer ch.Close()

	if err := sub.declare(ch); err != nil {
		return err
	}
	// One unacknowledged message at a time keeps ordering within a queue and bounds
	// what is redelivered after a crash.
	if err := ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set prefetch: %w", err)
	}
	deliveries, err := ch.ConsumeWithContext(ctx, sub.Queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", sub.Queue, err)
	}

	for d := range deliveries {
		b.handle(ctx, sub, h, d)
	}
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("delivery channel closed")
}

func (b *Broker) handle(ctx context.Context, sub Subscription, h Handler, d amqp.Delivery) {
	var env Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		b.settle(ctx, sub, d, fmt.Errorf("malformed envelope: %w", err), true)
		return
	}
	if err := env.Validate(); err != nil {
		b.settle(ctx, sub, d, err, true)
		return
	}

	err := b.run(ContextFor(ctx, env), h, env)
	if err == nil {
		b.ack(ctx, sub, d)
		return
	}
	if ctx.Err() != nil {
		// Shutting down: the failure is probably the cancelled context, not the message.
		// Hand it back without spending a retry.
		d.Nack(false, true)
		return
	}
	b.log.WarnContext(ctx, "event handler failed",
		"queue", sub.Queue, "event_id", env.ID, "type", env.Type,
		"correlation_id", env.CorrelationID, "error", err)
	b.settle(ctx, sub, d, err, isPermanent(err))
}

// run calls the handler and turns a panic into an error, so one bad message cannot
// crash the consumer or stay stuck at the head of the queue.
func (b *Broker) run(ctx context.Context, h Handler, env Envelope) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return h(ctx, env)
}

// settle moves a failed message to the retry queue or the DLQ and only then acknowledges
// the original, so a failure here cannot lose the message: it is requeued instead.
func (b *Broker) settle(ctx context.Context, sub Subscription, d amqp.Delivery, cause error, permanent bool) {
	attempts := retryCount(d.Headers)
	key := sub.dlqQueue()
	if !permanent && attempts < sub.MaxRetries {
		key = sub.retryQueue()
		attempts++
	}

	headers := amqp.Table{}
	for k, v := range d.Headers {
		headers[k] = v
	}
	headers[headerRetryCount] = int64(attempts)
	if key == sub.dlqQueue() {
		headers[headerError] = cause.Error()
		headers[headerFailedAt] = time.Now().UTC().Format(time.RFC3339)
	}

	err := b.publish(ctx, "", key, amqp.Publishing{
		ContentType:  d.ContentType,
		DeliveryMode: amqp.Persistent,
		MessageId:    d.MessageId,
		Type:         d.Type,
		Timestamp:    d.Timestamp,
		Headers:      headers,
		Body:         d.Body,
	})
	if err != nil {
		b.log.ErrorContext(ctx, "could not move failed message, requeueing",
			"queue", sub.Queue, "target", key, "error", err)
		d.Nack(false, true)
		return
	}
	if key == sub.dlqQueue() {
		b.log.ErrorContext(ctx, "message moved to dead-letter queue",
			"queue", sub.Queue, "message_id", d.MessageId, "error", cause)
	}
	b.ack(ctx, sub, d)
}

func (b *Broker) ack(ctx context.Context, sub Subscription, d amqp.Delivery) {
	if err := d.Ack(false); err != nil {
		// The connection died before the ack: the broker redelivers, which is safe
		// because handlers are idempotent.
		b.log.WarnContext(ctx, "ack failed", "queue", sub.Queue, "error", err)
	}
}

func retryCount(h amqp.Table) int {
	switch v := h[headerRetryCount].(type) {
	case int64:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	}
	return 0
}
