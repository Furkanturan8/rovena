package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Exchange is the single topic exchange every event is published to. The routing key
// is the event type, so a queue chooses what it receives by binding to event types.
const Exchange = "rovena.events"

const (
	dialTimeout = 5 * time.Second
	heartbeat   = 10 * time.Second
)

// Broker owns one RabbitMQ connection and reopens it when it drops, so a broker
// restart costs a few failed publishes and delayed deliveries, not a service restart.
type Broker struct {
	url string
	log *slog.Logger

	mu      sync.Mutex
	conn    *amqp.Connection
	pubCh   *amqp.Channel // confirm mode; guarded by mu, so publishes are serialized
	closing bool
}

// Dial connects to RabbitMQ and declares the event exchange.
func Dial(ctx context.Context, url string, log *slog.Logger) (*Broker, error) {
	if log == nil {
		log = slog.Default()
	}
	b := &Broker{url: url, log: log}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, err := b.publisherLocked(ctx); err != nil {
		return nil, err
	}
	return b, nil
}

// Close shuts the connection. Consume loops return once their context ends.
func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closing = true
	if b.conn == nil || b.conn.IsClosed() {
		return nil
	}
	return b.conn.Close()
}

// Publish sends the event and returns only after the broker has confirmed it is stored
// on a durable queue, so a nil error means the event will not be lost by a broker
// restart. An event routed to no queue is dropped by RabbitMQ: that is a normal
// publish/subscribe outcome, not an error.
func (b *Broker) Publish(ctx context.Context, env Envelope) error {
	if err := env.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	return b.publish(ctx, Exchange, env.Type, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    env.ID.String(),
		Type:         env.Type,
		Timestamp:    env.OccurredAt,
		Body:         body,
	})
}

func (b *Broker) publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch, err := b.publisherLocked(ctx)
	if err != nil {
		return err
	}
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, false, false, msg)
	if err != nil {
		b.dropPublisherLocked()
		return fmt.Errorf("publish %s: %w", key, err)
	}
	acked, err := confirm.WaitContext(ctx)
	if err != nil {
		// The outcome is unknown. Callers retry, which is why consumers must be idempotent.
		b.dropPublisherLocked()
		return fmt.Errorf("await confirm for %s: %w", key, err)
	}
	if !acked {
		return fmt.Errorf("broker rejected %s", key)
	}
	return nil
}

// connection returns the live connection, dialing a new one if needed.
func (b *Broker) connection(ctx context.Context) (*amqp.Connection, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.connectionLocked(ctx)
}

func (b *Broker) connectionLocked(ctx context.Context) (*amqp.Connection, error) {
	if b.closing {
		return nil, errors.New("broker closed")
	}
	if b.conn != nil && !b.conn.IsClosed() {
		return b.conn, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := amqp.DialConfig(b.url, amqp.Config{
		Heartbeat: heartbeat,
		Dial:      amqp.DefaultDial(dialTimeout),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}
	b.conn = conn
	b.pubCh = nil
	b.log.InfoContext(ctx, "rabbitmq connected")
	return conn, nil
}

func (b *Broker) publisherLocked(ctx context.Context) (*amqp.Channel, error) {
	if b.pubCh != nil && !b.pubCh.IsClosed() {
		return b.pubCh, nil
	}
	conn, err := b.connectionLocked(ctx)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open publish channel: %w", err)
	}
	if err := declareExchange(ch); err != nil {
		ch.Close()
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}
	b.pubCh = ch
	return ch, nil
}

// dropPublisherLocked discards the channel after a failure; the next publish opens a fresh one.
func (b *Broker) dropPublisherLocked() {
	if b.pubCh != nil {
		b.pubCh.Close()
		b.pubCh = nil
	}
}
