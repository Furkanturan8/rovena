//go:build integration

package consumer_test

// End-to-end: PaymentRequested goes into RabbitMQ, the real consumer, service,
// PostgreSQL repository, outbox and relay run, and the result event comes out.
//
//   PAYMENT_TEST_DATABASE_URL=postgres://... ROVENA_TEST_AMQP_URL=amqp://... \
//     go test -tags integration ./internal/transport/consumer
//
// Each test uses its own queues and its own event type for the command, so a Payment
// service that happens to be running against the same broker never sees these messages.
// Results are routed by the real PaymentSucceeded/PaymentFailed types and filtered by order.

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/payment/internal/infrastructure/provider"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository/postgres"
	"github.com/Furkanturan8/rovena/services/payment/internal/transport/consumer"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/outbox"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/uptrace/bun"
)

const maxAttempts = 3

type stack struct {
	t      *testing.T
	ctx    context.Context
	db     *bun.DB
	repo   *postgres.PaymentRepository
	broker *messaging.Broker
	conn   *amqp.Connection
	raw    *amqp.Channel
	cmd    string // routing key of the commands this test sends
	queue  string

	mu      sync.Mutex
	results map[uuid.UUID]chan messaging.Envelope
	orders  []uuid.UUID
}

func newStack(t *testing.T) *stack {
	t.Helper()
	dsn, amqpURL := os.Getenv("PAYMENT_TEST_DATABASE_URL"), os.Getenv("ROVENA_TEST_AMQP_URL")
	if dsn == "" || amqpURL == "" {
		t.Skip("PAYMENT_TEST_DATABASE_URL and ROVENA_TEST_AMQP_URL are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := messaging.Dial(ctx, amqpURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}

	id := uuid.NewString()[:8]
	s := &stack{
		t: t, ctx: ctx, db: db, repo: postgres.NewPaymentRepository(db), broker: broker, conn: conn, raw: raw,
		cmd: "test.PaymentRequested." + id, queue: "test.payment.requests." + id,
		results: map[uuid.UUID]chan messaging.Envelope{},
	}

	svc := application.NewPaymentService(s.repo, provider.NewFake(0), nil, application.Config{
		MaxAttempts: maxAttempts, ChargeTimeout: 2 * time.Second,
	})
	sub := consumer.Subscription(maxAttempts, 150*time.Millisecond)
	// The test runs on its own vhost, so the real RefundRequested type is safe to use as well.
	sub.Queue, sub.Types = s.queue, []string{s.cmd, events.RefundRequested}
	resultsSub := messaging.Subscription{
		Queue: "test.payment.results." + id, Types: []string{events.PaymentSucceeded, events.PaymentFailed, events.PaymentRefunded},
	}

	runCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, run := range []func(){
		func() {
			broker.Consume(runCtx, sub, s.asPaymentRequested(consumer.NewPaymentRequestedHandler(svc).Handle))
		},
		func() { broker.Consume(runCtx, resultsSub, s.collect) },
		func() { outbox.NewRelay(db, broker, nil).WithInterval(50 * time.Millisecond).Run(runCtx) },
	} {
		wg.Add(1)
		go func() { defer wg.Done(); run() }()
	}

	t.Cleanup(func() {
		stop()
		wg.Wait()
		for _, q := range []string{s.queue, s.queue + ".retry", s.queue + ".dlq", resultsSub.Queue, resultsSub.Queue + ".dlq"} {
			raw.QueueDelete(q, false, false, false)
		}
		for _, o := range s.orders {
			db.ExecContext(ctx, "DELETE FROM outbox_events WHERE envelope->'payload'->>'order_id' = ?", o.String())
			db.ExecContext(ctx, "DELETE FROM payments WHERE order_id = ?", o)
		}
		conn.Close()
		broker.Close()
		db.Close()
	})

	// Both queues must exist before the first publish, or the message would be unrouted.
	s.waitQueue(s.queue)
	s.waitQueue(resultsSub.Queue)
	return s
}

// asPaymentRequested lets the handler see the test's private command type as the real
// PaymentRequested: the handler chooses what to do by event type, and a unique routing key per test
// is what keeps these messages away from a Payment service that runs on the same broker.
func (s *stack) asPaymentRequested(h messaging.Handler) messaging.Handler {
	return func(ctx context.Context, env messaging.Envelope) error {
		if env.Type == s.cmd {
			env.Type = events.PaymentRequested
		}
		return h(ctx, env)
	}
}

func (s *stack) waitQueue(name string) {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ch, err := s.conn.Channel()
		if err == nil {
			_, qerr := ch.QueueDeclarePassive(name, true, false, false, false, nil)
			ch.Close()
			if qerr == nil {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatalf("queue %s never appeared", name)
}

// collect hands result events to whichever test is waiting for that order.
func (s *stack) collect(_ context.Context, env messaging.Envelope) error {
	var p struct {
		OrderID uuid.UUID `json:"order_id"`
	}
	if err := env.DecodePayload(&p); err != nil {
		return nil
	}
	s.mu.Lock()
	ch := s.results[p.OrderID]
	s.mu.Unlock()
	if ch != nil {
		ch <- env
	}
	return nil
}

type order struct {
	id         uuid.UUID
	customer   uuid.UUID
	results    chan messaging.Envelope
	request    events.PaymentRequestedPayload
	correlated string
}

func (s *stack) newOrder(method string) *order {
	o := &order{id: uuid.New(), customer: uuid.New(), results: make(chan messaging.Envelope, 20)}
	o.request = events.PaymentRequestedPayload{
		OrderID: o.id, CustomerID: o.customer, IdempotencyKey: "key-" + uuid.NewString(),
		AmountCents: 459900, Currency: "TRY", PaymentMethod: method,
	}
	o.correlated = "flow-" + uuid.NewString()[:8]
	s.mu.Lock()
	s.results[o.id] = o.results
	s.orders = append(s.orders, o.id)
	s.mu.Unlock()
	return o
}

func (s *stack) send(o *order, payload events.PaymentRequestedPayload) {
	s.t.Helper()
	ctx := messaging.WithCorrelationID(s.ctx, o.correlated)
	env, err := messaging.NewEnvelope(ctx, s.cmd, events.PaymentRequestedVersion, payload)
	if err != nil {
		s.t.Fatal(err)
	}
	if err := s.broker.Publish(s.ctx, env); err != nil {
		s.t.Fatal(err)
	}
}

func (s *stack) expect(o *order, eventType string) messaging.Envelope {
	s.t.Helper()
	select {
	case env := <-o.results:
		if env.Type != eventType {
			s.t.Fatalf("got %s, want %s: %s", env.Type, eventType, env.Payload)
		}
		if env.CorrelationID != o.correlated {
			s.t.Fatalf("the result left the flow: correlation %q, want %q", env.CorrelationID, o.correlated)
		}
		return env
	case <-time.After(15 * time.Second):
		s.t.Fatalf("no %s event arrived", eventType)
		return messaging.Envelope{}
	}
}

func (s *stack) expectNothing(o *order, wait time.Duration) {
	s.t.Helper()
	select {
	case env := <-o.results:
		s.t.Fatalf("unexpected %s event: %s", env.Type, env.Payload)
	case <-time.After(wait):
	}
}

func (s *stack) payment(o *order) *domain.Payment {
	s.t.Helper()
	p, err := s.repo.LatestByOrder(s.ctx, o.id)
	if err != nil {
		s.t.Fatal(err)
	}
	return p
}

func (s *stack) count(o *order) int {
	s.t.Helper()
	_, total, err := s.repo.List(s.ctx, domain.Filter{OrderID: &o.id, Page: 1, PageSize: 50})
	if err != nil {
		s.t.Fatal(err)
	}
	return total
}

func (s *stack) waitDLQ() amqp.Delivery {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		d, ok, err := s.raw.Get(s.queue+".dlq", true)
		if err != nil {
			s.t.Fatal(err)
		}
		if ok {
			return d
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatal("nothing reached the dead-letter queue")
	return amqp.Delivery{}
}

func TestApprovedPaymentPublishesPaymentSucceeded(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodApproved)

	s.send(o, o.request)

	env := s.expect(o, events.PaymentSucceeded)
	var got events.PaymentSucceededPayload
	if err := env.DecodePayload(&got); err != nil {
		t.Fatal(err)
	}
	p := s.payment(o)
	if got.PaymentID != p.ID || got.OrderID != o.id || got.AmountCents != 459900 || got.ProviderRef == "" || got.ProviderRef != p.ProviderRef {
		t.Fatalf("event %+v does not match payment %+v", got, p)
	}
	if p.Status != domain.StatusSucceeded || p.Attempts != 1 || p.CustomerID != o.customer {
		t.Fatalf("unexpected payment: %+v", p)
	}
}

func TestDeclinedPaymentPublishesPaymentFailedWithReason(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodInsufficientFunds)

	s.send(o, o.request)

	env := s.expect(o, events.PaymentFailed)
	var got events.PaymentFailedPayload
	if err := env.DecodePayload(&got); err != nil {
		t.Fatal(err)
	}
	if got.FailureCode != domain.CodeInsufficientFunds || got.FailureMessage == "" || got.OrderID != o.id {
		t.Fatalf("unexpected event: %+v", got)
	}
	if p := s.payment(o); p.Status != domain.StatusFailed || p.Attempts != 1 {
		t.Fatalf("unexpected payment: %+v", p)
	}
}

func TestDuplicateRequestsChargeOnceAndAnswerEachTime(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodApproved)

	// The same intent arrives three times as three different messages (Order retried).
	for i := 0; i < 3; i++ {
		s.send(o, o.request)
	}

	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		env := s.expect(o, events.PaymentSucceeded)
		var got events.PaymentSucceededPayload
		env.DecodePayload(&got)
		ids = append(ids, got.PaymentID)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("results name different payments: %v", ids)
		}
	}
	if s.count(o) != 1 {
		t.Fatalf("%d payments exist for one idempotency key", s.count(o))
	}
	if p := s.payment(o); p.Attempts != 1 {
		t.Fatalf("the provider was called %d times for one payment", p.Attempts)
	}
}

func TestRedeliveredMessageDoesNotChargeTwice(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodApproved)
	s.send(o, o.request)
	s.expect(o, events.PaymentSucceeded)

	// Same payload again after completion, as a broker redelivery or an Order retry would do.
	s.send(o, o.request)
	s.expect(o, events.PaymentSucceeded)

	if p := s.payment(o); s.count(o) != 1 || p.Attempts != 1 {
		t.Fatalf("payments=%d attempts=%d", s.count(o), p.Attempts)
	}
}

func TestFlakyProviderIsRetriedUntilItAnswers(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodFlaky)

	s.send(o, o.request)

	s.expect(o, events.PaymentSucceeded)
	if p := s.payment(o); p.Status != domain.StatusSucceeded || p.Attempts != 3 {
		t.Fatalf("expected success on the third provider call: %+v", p)
	}
}

func TestProviderOutageEndsWithPaymentFailedNotSilence(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodProviderError)

	s.send(o, o.request)

	env := s.expect(o, events.PaymentFailed)
	var got events.PaymentFailedPayload
	env.DecodePayload(&got)
	if got.FailureCode != domain.CodeProviderUnavailable {
		t.Fatalf("unexpected failure: %+v", got)
	}
	if p := s.payment(o); p.Attempts != maxAttempts || p.Status != domain.StatusFailed {
		t.Fatalf("unexpected payment: %+v", p)
	}
	// The order got its answer, so nothing is parked in the dead-letter queue.
	if _, ok, _ := s.raw.Get(s.queue+".dlq", false); ok {
		t.Fatal("an answered request must not end up in the DLQ")
	}
}

func TestInvalidRequestGoesToDeadLetterQueueWithoutAPayment(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodApproved)
	bad := o.request
	bad.AmountCents = 0

	s.send(o, bad)

	d := s.waitDLQ()
	if msg, _ := d.Headers["x-error"].(string); msg == "" {
		t.Fatal("the DLQ message should say why it failed")
	}
	if s.count(o) != 0 {
		t.Fatal("an invalid request must not create a payment")
	}
	s.expectNothing(o, 300*time.Millisecond)
}

func TestReusedKeyForADifferentAmountGoesToDeadLetterQueue(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodApproved)
	s.send(o, o.request)
	s.expect(o, events.PaymentSucceeded)

	changed := o.request
	changed.AmountCents += 100
	s.send(o, changed)

	s.waitDLQ()
	s.expectNothing(o, 300*time.Millisecond)
	if p := s.payment(o); p.AmountCents != 459900 || s.count(o) != 1 {
		t.Fatalf("the original payment must be untouched: %+v", p)
	}
}

func TestSecondPaymentForAPaidOrderFailsWithoutCharging(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodApproved)
	s.send(o, o.request)
	s.expect(o, events.PaymentSucceeded)

	again := o.request
	again.IdempotencyKey = "key-" + uuid.NewString() // a new intent for the same order
	s.send(o, again)

	env := s.expect(o, events.PaymentFailed)
	var got events.PaymentFailedPayload
	env.DecodePayload(&got)
	if got.FailureCode != domain.CodeOrderAlreadyPaid {
		t.Fatalf("unexpected failure: %+v", got)
	}
	if s.count(o) != 2 {
		t.Fatalf("expected the two attempts to be recorded, got %d", s.count(o))
	}
	if p := s.payment(o); p.Attempts != 0 {
		t.Fatalf("the provider must not be called for an already paid order, attempts=%d", p.Attempts)
	}
}

func (s *stack) sendRefund(o *order, orderID uuid.UUID) {
	s.t.Helper()
	ctx := messaging.WithCorrelationID(s.ctx, o.correlated)
	env, err := messaging.NewEnvelope(ctx, events.RefundRequested, events.RefundRequestedVersion,
		events.RefundRequestedPayload{OrderID: orderID, CustomerID: o.customer, Reason: "payment_timeout"})
	if err != nil {
		s.t.Fatal(err)
	}
	if err := s.broker.Publish(s.ctx, env); err != nil {
		s.t.Fatal(err)
	}
}

func TestRefundRequestGivesTheMoneyBackAndAnnouncesIt(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodApproved)
	s.send(o, o.request)
	s.expect(o, events.PaymentSucceeded)

	s.sendRefund(o, o.id)

	env := s.expect(o, events.PaymentRefunded)
	var got events.PaymentRefundedPayload
	if err := env.DecodePayload(&got); err != nil {
		t.Fatal(err)
	}
	p := s.payment(o)
	if got.PaymentID != p.ID || got.OrderID != o.id || got.AmountCents != 459900 || got.RefundRef == "" || got.RefundRef != p.RefundRef {
		t.Fatalf("event %+v does not match payment %+v", got, p)
	}
	if p.Status != domain.StatusRefunded || p.RefundedAt == nil {
		t.Fatalf("payment = %+v", p)
	}

	// The same request again, as a retry or a duplicate would: answered again, refunded once.
	s.sendRefund(o, o.id)
	again := s.expect(o, events.PaymentRefunded)
	var second events.PaymentRefundedPayload
	_ = again.DecodePayload(&second)
	if second.RefundRef != got.RefundRef || s.count(o) != 1 {
		t.Fatalf("a repeated refund request changed the result: %+v vs %+v, %d payments", second, got, s.count(o))
	}
}

func TestRefundRequestForAnOrderThatWasNeverPaidGoesToTheDeadLetterQueue(t *testing.T) {
	s := newStack(t)
	o := s.newOrder(provider.MethodDeclined)
	s.send(o, o.request)
	s.expect(o, events.PaymentFailed)

	s.sendRefund(o, o.id)

	d := s.waitDLQ()
	if msg, _ := d.Headers["x-error"].(string); msg == "" {
		t.Fatal("the DLQ message should say why it failed")
	}
	s.expectNothing(o, 300*time.Millisecond)
	if p := s.payment(o); p.Status != domain.StatusFailed {
		t.Fatalf("payment = %+v", p)
	}
}
