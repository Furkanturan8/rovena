package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Furkanturan8/rovena/shared/telemetry"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
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

// IsPermanent reports whether err was marked with Permanent.
func IsPermanent(err error) bool {
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
	defer func() { _ = ch.Close() }()

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
		b.settle(ctx, sub, d, "unknown", fmt.Errorf("malformed envelope: %w", err), true)
		return
	}
	if err := env.Validate(); err != nil {
		b.settle(ctx, sub, d, env.Type, err, true)
		return
	}

	// Continue the trace of the publish span (header), falling back to the one in the envelope.
	parent, _ := d.Headers[headerTraceparent].(string)
	if parent == "" {
		parent = env.Traceparent
	}
	spanCtx, span := telemetry.Tracer().Start(telemetry.WithTraceparent(ctx, parent), "process "+sub.Queue,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", sub.Queue),
			attribute.String("messaging.message.id", env.ID.String()),
			attribute.String("event.type", env.Type),
			attribute.String("correlation_id", env.CorrelationID),
			attribute.Int("messaging.retry_count", retryCount(d.Headers)),
		))
	defer span.End()

	started := time.Now()
	err := b.run(ContextFor(spanCtx, env), h, env)
	handled.Record(ctx, time.Since(started).Seconds(),
		metric.WithAttributes(attribute.String("queue", sub.Queue), attribute.String("type", env.Type)))
	if err == nil {
		consumed.Add(ctx, 1, metric.WithAttributes(attribute.String("queue", sub.Queue), attribute.String("type", env.Type), attribute.String("result", "ok")))
		b.ack(ctx, sub, d)
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	if ctx.Err() != nil {
		// Shutting down: the failure is probably the cancelled context, not the message.
		// Hand it back without spending a retry.
		_ = d.Nack(false, true)
		return
	}
	b.log.WarnContext(ctx, "event handler failed",
		"queue", sub.Queue, "event_id", env.ID, "type", env.Type,
		"correlation_id", env.CorrelationID, "error", err)
	b.settle(spanCtx, sub, d, env.Type, err, IsPermanent(err))
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
func (b *Broker) settle(ctx context.Context, sub Subscription, d amqp.Delivery, eventType string, cause error, permanent bool) {
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
		_ = d.Nack(false, true)
		return
	}
	result := "retried"
	if key == sub.dlqQueue() {
		result = "dead_lettered"
		b.log.ErrorContext(ctx, "message moved to dead-letter queue",
			"queue", sub.Queue, "message_id", d.MessageId, "error", cause)
	}
	consumed.Add(ctx, 1, metric.WithAttributes(attribute.String("queue", sub.Queue), attribute.String("type", eventType), attribute.String("result", result)))
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
