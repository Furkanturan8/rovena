//go:build integration

package messaging_test

// Runs against a real RabbitMQ:
//   ROVENA_TEST_AMQP_URL=amqp://user:pass@localhost:5672/ go test -tags integration ./messaging
// Every test uses its own queue names and event type, and deletes its queues afterwards.

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type harness struct {
	ctx    context.Context
	url    string
	broker *messaging.Broker
	conn   *amqp.Connection // a second, independent connection used to inspect queues
	raw    *amqp.Channel
	sub    messaging.Subscription
	typ    string
}

func setup(t *testing.T, maxRetries int) *harness {
	t.Helper()
	url := os.Getenv("ROVENA_TEST_AMQP_URL")
	if url == "" {
		t.Skip("ROVENA_TEST_AMQP_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	broker, err := messaging.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}

	id := uuid.NewString()[:8]
	h := &harness{
		ctx: ctx, url: url, broker: broker, conn: conn, raw: raw,
		typ: "test.event." + id,
		sub: messaging.Subscription{
			Queue:      "test.queue." + id,
			Types:      []string{"test.event." + id},
			MaxRetries: maxRetries,
			RetryDelay: 200 * time.Millisecond,
		},
	}
	t.Cleanup(func() {
		for _, q := range []string{h.sub.Queue, h.sub.Queue + ".retry", h.sub.Queue + ".dlq"} {
			raw.QueueDelete(q, false, false, false)
		}
		conn.Close()
		broker.Close()
	})
	return h
}

func (h *harness) consume(t *testing.T, handler messaging.Handler) {
	t.Helper()
	cctx, cancel := context.WithCancel(h.ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := h.broker.Consume(cctx, h.sub, handler); err != nil {
			t.Error(err)
		}
	}()
	t.Cleanup(func() { cancel(); <-done })
	// The queue is declared by Consume; wait until it exists so publishes are routed.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ch, err := h.conn.Channel()
		if err == nil {
			_, qerr := ch.QueueDeclarePassive(h.sub.Queue, true, false, false, false, nil)
			ch.Close()
			if qerr == nil {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("queue was never declared")
}

func (h *harness) publish(t *testing.T, payload any) messaging.Envelope {
	t.Helper()
	env, err := messaging.NewEnvelope(h.ctx, h.typ, 1, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.broker.Publish(h.ctx, env); err != nil {
		t.Fatal(err)
	}
	return env
}

// waitDLQ returns the first message to reach the dead-letter queue.
func (h *harness) waitDLQ(t *testing.T) amqp.Delivery {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		d, ok, err := h.raw.Get(h.sub.Queue+".dlq", true)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			return d
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("nothing reached the dead-letter queue")
	return amqp.Delivery{}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestPublishedEventReachesConsumerWithMetadata(t *testing.T) {
	h := setup(t, 0)
	got := make(chan messaging.Envelope, 1)
	h.consume(t, func(ctx context.Context, env messaging.Envelope) error {
		got <- env
		return nil
	})

	ctx := messaging.WithRequestID(messaging.WithCorrelationID(h.ctx, "flow-7"), "req-7")
	env, _ := messaging.NewEnvelope(ctx, h.typ, 1, map[string]string{"order_id": "o1"})
	if err := h.broker.Publish(h.ctx, env); err != nil {
		t.Fatal(err)
	}

	select {
	case e := <-got:
		if e.ID != env.ID || e.CorrelationID != "flow-7" || e.RequestID != "req-7" {
			t.Fatalf("metadata lost: %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("event never arrived")
	}
}

func TestFailingHandlerIsRetriedUntilItSucceeds(t *testing.T) {
	h := setup(t, 5)
	var calls atomic.Int32
	h.consume(t, func(ctx context.Context, env messaging.Envelope) error {
		if calls.Add(1) < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	h.publish(t, struct{}{})

	waitFor(t, "third attempt", func() bool { return calls.Load() == 3 })
	time.Sleep(600 * time.Millisecond) // no further deliveries once it succeeded
	if n := calls.Load(); n != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", n)
	}
}

func TestPoisonMessageLandsInDeadLetterQueueAfterRetries(t *testing.T) {
	h := setup(t, 2)
	var calls atomic.Int32
	h.consume(t, func(ctx context.Context, env messaging.Envelope) error {
		calls.Add(1)
		return errors.New("always broken")
	})

	env := h.publish(t, struct{}{})

	d := h.waitDLQ(t)
	if n := calls.Load(); n != 3 { // the first attempt plus MaxRetries redeliveries
		t.Fatalf("expected 3 attempts, got %d", n)
	}
	if d.MessageId != env.ID.String() {
		t.Fatalf("dead-lettered a different message: %s", d.MessageId)
	}
	if msg, _ := d.Headers["x-error"].(string); msg != "always broken" {
		t.Fatalf("x-error header = %q", msg)
	}
}

func TestPermanentErrorSkipsRetries(t *testing.T) {
	h := setup(t, 5)
	var calls atomic.Int32
	h.consume(t, func(ctx context.Context, env messaging.Envelope) error {
		calls.Add(1)
		return messaging.Permanent(errors.New("invalid payload"))
	})

	h.publish(t, struct{}{})

	h.waitDLQ(t)
	if n := calls.Load(); n != 1 {
		t.Fatalf("a permanent error must not be retried, got %d attempts", n)
	}
}

func TestPanickingHandlerDoesNotKillConsumer(t *testing.T) {
	h := setup(t, 0)
	var calls atomic.Int32
	h.consume(t, func(ctx context.Context, env messaging.Envelope) error {
		if calls.Add(1) == 1 {
			panic("boom")
		}
		return nil
	})

	h.publish(t, struct{}{})
	h.waitDLQ(t)

	h.publish(t, struct{}{})
	waitFor(t, "second message handled after the panic", func() bool { return calls.Load() == 2 })
}

func TestMalformedMessageGoesStraightToDeadLetterQueue(t *testing.T) {
	h := setup(t, 5)
	var calls atomic.Int32
	h.consume(t, func(ctx context.Context, env messaging.Envelope) error {
		calls.Add(1)
		return nil
	})

	err := h.raw.PublishWithContext(h.ctx, messaging.Exchange, h.typ, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         []byte("this is not json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	h.waitDLQ(t)
	if calls.Load() != 0 {
		t.Fatal("the handler must never see a malformed message")
	}
}

func TestEventsPublishedBeforeTheConsumerStartsAreKept(t *testing.T) {
	h := setup(t, 0)
	got := make(chan uuid.UUID, 1)

	// Start once so the queue exists, then stop and publish while nobody listens.
	cctx, cancel := context.WithCancel(h.ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.broker.Consume(cctx, h.sub, func(context.Context, messaging.Envelope) error { return nil })
	}()
	waitFor(t, "queue", func() bool {
		ch, err := h.conn.Channel()
		if err != nil {
			return false
		}
		defer ch.Close()
		_, err = ch.QueueDeclarePassive(h.sub.Queue, true, false, false, false, nil)
		return err == nil
	})
	cancel()
	<-done

	env := h.publish(t, struct{}{})

	h.consume(t, func(ctx context.Context, e messaging.Envelope) error {
		got <- e.ID
		return nil
	})
	select {
	case id := <-got:
		if id != env.ID {
			t.Fatal("received a different event")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("event published while the consumer was down was lost")
	}
}
