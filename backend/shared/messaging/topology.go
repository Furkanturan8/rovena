package messaging

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Subscription describes one consumer's queue and what happens to messages that fail.
//
// For a queue "payment.requests" the broker holds three queues:
//
//	payment.requests        the work queue, bound to the event types below
//	payment.requests.retry  no consumer; messages wait here for RetryDelay and are then
//	                        dead-lettered back into the work queue
//	payment.requests.dlq    messages that will not succeed, kept for a human to inspect
//
// RabbitMQ fixes queue arguments at creation: changing RetryDelay for an existing queue
// needs the queue deleted first, otherwise declaring it fails with PRECONDITION_FAILED.
type Subscription struct {
	Queue string
	// Types are the event types (routing keys) the queue receives.
	Types []string
	// MaxRetries is how many times a failing message is redelivered before it goes to the DLQ.
	MaxRetries int
	RetryDelay time.Duration
}

func (s Subscription) retryQueue() string { return s.Queue + ".retry" }
func (s Subscription) dlqQueue() string   { return s.Queue + ".dlq" }

func (s Subscription) validate() error {
	switch {
	case s.Queue == "":
		return fmt.Errorf("subscription: queue name is required")
	case len(s.Types) == 0:
		return fmt.Errorf("subscription %s: at least one event type is required", s.Queue)
	case s.MaxRetries < 0:
		return fmt.Errorf("subscription %s: MaxRetries must not be negative", s.Queue)
	case s.MaxRetries > 0 && s.RetryDelay <= 0:
		return fmt.Errorf("subscription %s: RetryDelay is required when MaxRetries > 0", s.Queue)
	}
	return nil
}

func declareExchange(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", Exchange, err)
	}
	return nil
}

// declare is idempotent: services call it on every start, so a consumer never depends
// on another service having created the queues first.
func (s Subscription) declare(ch *amqp.Channel) error {
	if err := declareExchange(ch); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(s.dlqQueue(), true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare %s: %w", s.dlqQueue(), err)
	}
	if _, err := ch.QueueDeclare(s.Queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare %s: %w", s.Queue, err)
	}
	for _, t := range s.Types {
		if err := ch.QueueBind(s.Queue, t, Exchange, false, nil); err != nil {
			return fmt.Errorf("bind %s to %s: %w", s.Queue, t, err)
		}
	}
	if s.MaxRetries == 0 {
		return nil
	}
	// Expired messages are dead-lettered through the default exchange straight back
	// into the work queue.
	_, err := ch.QueueDeclare(s.retryQueue(), true, false, false, false, amqp.Table{
		"x-message-ttl":             s.RetryDelay.Milliseconds(),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": s.Queue,
	})
	if err != nil {
		return fmt.Errorf("declare %s: %w", s.retryQueue(), err)
	}
	return nil
}
