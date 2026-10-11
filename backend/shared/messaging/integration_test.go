//go:build integration

package messaging_test

// Runs against a real RabbitMQ:
//   ROVENA_TEST_AMQP_URL=amqp://user:pass@localhost:5672/ go test -tags integration ./messaging
// Every test uses its own queue names and event type, and deletes its queues afterwards.

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/telemetry"
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

var telemetryOnce sync.Once

func setupTelemetry(t *testing.T) {
	t.Helper()
	telemetryOnce.Do(func() {
		if _, err := telemetry.Setup(context.Background(), "messaging-test", nil); err != nil {
			t.Fatal(err)
		}
	})
}

// The relay publishes with a context that knows nothing about the request, and the consumer is
// another process. The trace still has to run request -> publish -> handler -> the next event.
func TestTraceSurvivesTheOutboxTheBrokerAndTheNextHop(t *testing.T) {
	setupTelemetry(t)
	h := setup(t, 0)
	type seen struct{ trace, next string }
	got := make(chan seen, 1)
	h.consume(t, func(ctx context.Context, env messaging.Envelope) error {
		follow, err := messaging.NewEnvelope(ctx, h.typ+".next", 1, struct{}{})
		if err != nil {
			return err
		}
		got <- seen{trace: telemetry.TraceID(ctx), next: follow.Traceparent}
		return nil
	})

	reqCtx, span := telemetry.Tracer().Start(h.ctx, "POST /checkout")
	env, err := messaging.NewEnvelope(reqCtx, h.typ, 1, struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	span.End() // the request is over long before the event is sent
	if err := h.broker.Publish(context.Background(), env); err != nil {
		t.Fatal(err)
	}

	select {
	case s := <-got:
		want := telemetry.TraceID(reqCtx)
		if s.trace != want {
			t.Fatalf("handler ran in trace %q, the request was %q", s.trace, want)
		}
		if !strings.Contains(s.next, want) {
			t.Fatalf("the follow-up event left the trace: %q does not contain %q", s.next, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("event never arrived")
	}
}

func TestConsumeMetricsCountEveryOutcome(t *testing.T) {
	setupTelemetry(t)
	h := setup(t, 1)
	var calls atomic.Int32
	h.consume(t, func(context.Context, messaging.Envelope) error {
		if calls.Add(1) == 1 {
			return errors.New("first time fails")
		}
		return nil
	})
	h.publish(t, struct{}{})
	h.publish(t, struct{}{}) // handled fine
	scrape := func() string {
		rec := httptest.NewRecorder()
		telemetry.MetricsHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
		return rec.Body.String()
	}
	has := func(out, result string) bool {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "messaging_events_consumed_total") && strings.Contains(line, `queue="`+h.sub.Queue+`"`) && strings.Contains(line, `result="`+result+`"`) {
				return true
			}
		}
		return false
	}
	// Counters are updated just after the handler returns; give them a moment.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && (!has(scrape(), "ok") || !has(scrape(), "retried")) {
		time.Sleep(100 * time.Millisecond)
	}
	out := scrape()
	for _, result := range []string{"ok", "retried"} {
		if !has(out, result) {
			t.Errorf("no messaging_events_consumed_total for result=%s on %s", result, h.sub.Queue)
		}
	}
	if !strings.Contains(out, "messaging_events_published_total") || !strings.Contains(out, "messaging_handler_duration_seconds_bucket") {
		t.Error("publish counter or handler histogram missing")
	}
}
