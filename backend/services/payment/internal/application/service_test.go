package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
)

var errOutage = errors.New("provider outage")

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newService(repo *memRepo, prov application.Provider, maxAttempts int) *application.PaymentService {
	return application.NewPaymentService(repo, prov, quietLog(), application.Config{
		MaxAttempts: maxAttempts, ChargeTimeout: time.Second,
	})
}

func request(key string) application.ProcessInput {
	return application.ProcessInput{
		OrderID: uuid.New(), CustomerID: uuid.New(), IdempotencyKey: key,
		AmountCents: 459900, Currency: "TRY", PaymentMethod: "tok_visa",
	}
}

func mustSucceed(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSuccessfulChargeFinishesPaymentAndQueuesEvent(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	in := request("key-1")

	mustSucceed(t, newService(repo, prov, 3).Process(context.Background(), in))

	p := repo.only()
	if p.Status != domain.StatusSucceeded || p.ProviderRef != "ref_key-1" || p.CompletedAt == nil || p.Attempts != 1 {
		t.Fatalf("unexpected payment: %+v", p)
	}
	evs := repo.events()
	if len(evs) != 1 || evs[0].Type != events.PaymentSucceeded {
		t.Fatalf("expected one PaymentSucceeded, got %+v", evs)
	}
	var payload events.PaymentSucceededPayload
	mustSucceed(t, evs[0].DecodePayload(&payload))
	if payload.PaymentID != p.ID || payload.OrderID != in.OrderID || payload.AmountCents != in.AmountCents || payload.ProviderRef != "ref_key-1" {
		t.Fatalf("payload does not match the payment: %+v", payload)
	}
}

func TestDeclinedChargeFailsWithReason(t *testing.T) {
	repo := newMemRepo()
	prov := newProvider(func(int, domain.ChargeRequest) (domain.ChargeResult, error) {
		return domain.ChargeResult{DeclineCode: domain.CodeInsufficientFunds, DeclineMessage: "no funds"}, nil
	})

	mustSucceed(t, newService(repo, prov, 3).Process(context.Background(), request("key-1")))

	p := repo.only()
	if p.Status != domain.StatusFailed || p.FailureCode != domain.CodeInsufficientFunds || p.ProviderRef != "" {
		t.Fatalf("unexpected payment: %+v", p)
	}
	evs := repo.events()
	if len(evs) != 1 || evs[0].Type != events.PaymentFailed {
		t.Fatalf("expected one PaymentFailed, got %+v", evs)
	}
	var payload events.PaymentFailedPayload
	mustSucceed(t, evs[0].DecodePayload(&payload))
	if payload.FailureCode != domain.CodeInsufficientFunds || payload.FailureMessage != "no funds" {
		t.Fatalf("payload: %+v", payload)
	}
	if prov.callCount() != 1 {
		t.Fatalf("a decline is final and must not be retried, provider called %d times", prov.callCount())
	}
}

func TestEventStaysInTheCorrelationOfTheRequest(t *testing.T) {
	repo := newMemRepo()
	ctx := messaging.WithRequestID(messaging.WithCorrelationID(context.Background(), "flow-9"), "req-9")

	mustSucceed(t, newService(repo, newProvider(approve), 3).Process(ctx, request("key-1")))

	ev := repo.events()[0]
	if ev.CorrelationID != "flow-9" || ev.RequestID != "req-9" {
		t.Fatalf("event left the flow: %+v", ev)
	}
}

func TestRepeatedRequestChargesOnceAndSendsTheResultAgain(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc := newService(repo, prov, 3)
	in := request("key-1")

	for i := 0; i < 3; i++ {
		mustSucceed(t, svc.Process(context.Background(), in))
	}

	if repo.count() != 1 || prov.callCount() != 1 || len(prov.charged) != 1 {
		t.Fatalf("expected a single payment and charge, got %d payments, %d provider calls", repo.count(), prov.callCount())
	}
	// The first completion queued the event; each repeat re-sends the stored result,
	// so an Order that missed the first one still learns the outcome.
	evs := repo.events()
	if len(evs) != 3 {
		t.Fatalf("expected 3 result events, got %d", len(evs))
	}
	first := evs[0]
	var want, got events.PaymentSucceededPayload
	mustSucceed(t, first.DecodePayload(&want))
	for _, e := range evs[1:] {
		if e.Type != events.PaymentSucceeded || e.ID == first.ID {
			t.Fatalf("a repeat must be a new event of the same kind: %+v", e)
		}
		mustSucceed(t, e.DecodePayload(&got))
		if got != want {
			t.Fatalf("repeated result differs: %+v vs %+v", got, want)
		}
	}
}

func TestRepeatedFailedRequestRepeatsTheFailure(t *testing.T) {
	repo := newMemRepo()
	prov := newProvider(func(int, domain.ChargeRequest) (domain.ChargeResult, error) {
		return domain.ChargeResult{DeclineCode: domain.CodeCardDeclined, DeclineMessage: "declined"}, nil
	})
	svc := newService(repo, prov, 3)
	in := request("key-1")

	mustSucceed(t, svc.Process(context.Background(), in))
	mustSucceed(t, svc.Process(context.Background(), in))

	if prov.callCount() != 1 {
		t.Fatalf("a failed payment must not be charged again under the same key, calls=%d", prov.callCount())
	}
	for _, e := range repo.events() {
		if e.Type != events.PaymentFailed {
			t.Fatalf("unexpected event %s", e.Type)
		}
	}
}

func TestReusedKeyForADifferentChargeIsRejected(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc := newService(repo, prov, 3)
	in := request("key-1")
	mustSucceed(t, svc.Process(context.Background(), in))

	for name, mutate := range map[string]func(*application.ProcessInput){
		"amount":   func(i *application.ProcessInput) { i.AmountCents++ },
		"order":    func(i *application.ProcessInput) { i.OrderID = uuid.New() },
		"customer": func(i *application.ProcessInput) { i.CustomerID = uuid.New() },
	} {
		t.Run(name, func(t *testing.T) {
			other := in
			mutate(&other)
			err := svc.Process(context.Background(), other)
			if !errors.Is(err, domain.ErrIdempotencyKeyReused) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if prov.callCount() != 1 || len(repo.events()) != 1 {
		t.Fatal("a rejected request must not charge or publish anything")
	}
}

func TestProviderOutageRetriesThenFailsThePayment(t *testing.T) {
	repo := newMemRepo()
	prov := newProvider(func(int, domain.ChargeRequest) (domain.ChargeResult, error) {
		return domain.ChargeResult{}, errOutage
	})
	svc := newService(repo, prov, 3)
	in := request("key-1")

	for attempt := 1; attempt <= 2; attempt++ {
		err := svc.Process(context.Background(), in)
		if !errors.Is(err, errOutage) {
			t.Fatalf("attempt %d: expected the outage to be returned for a retry, got %v", attempt, err)
		}
		if p := repo.only(); p.Status != domain.StatusPending || p.Attempts != attempt {
			t.Fatalf("attempt %d: payment should still be pending: %+v", attempt, p)
		}
		if len(repo.events()) != 0 {
			t.Fatal("no result may be published while the outcome is unknown")
		}
	}

	// The last allowed attempt turns the unknown into a definite answer for the order.
	mustSucceed(t, svc.Process(context.Background(), in))
	p := repo.only()
	if p.Status != domain.StatusFailed || p.FailureCode != domain.CodeProviderUnavailable || p.Attempts != 3 {
		t.Fatalf("unexpected payment: %+v", p)
	}
	if evs := repo.events(); len(evs) != 1 || evs[0].Type != events.PaymentFailed {
		t.Fatalf("expected one PaymentFailed, got %+v", evs)
	}
}

func TestPaymentSucceedsWhenProviderRecoversInTime(t *testing.T) {
	repo := newMemRepo()
	prov := newProvider(func(call int, req domain.ChargeRequest) (domain.ChargeResult, error) {
		if call < 3 {
			return domain.ChargeResult{}, errOutage
		}
		return approve(call, req)
	})
	svc := newService(repo, prov, 3)
	in := request("key-1")

	for i := 0; i < 2; i++ {
		if err := svc.Process(context.Background(), in); err == nil {
			t.Fatal("expected a retryable error")
		}
	}
	mustSucceed(t, svc.Process(context.Background(), in))

	if p := repo.only(); p.Status != domain.StatusSucceeded || p.Attempts != 3 {
		t.Fatalf("unexpected payment: %+v", p)
	}
}

func TestSlowProviderIsCutOffAndRetried(t *testing.T) {
	repo := &memRepo{payments: map[uuid.UUID]*domain.Payment{}}
	blocked := blockingProvider{}
	svc := application.NewPaymentService(repo, blocked, quietLog(), application.Config{
		MaxAttempts: 3, ChargeTimeout: 20 * time.Millisecond,
	})

	start := time.Now()
	err := svc.Process(context.Background(), request("key-1"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a timeout, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("the timeout did not bound the provider call")
	}
	if p := repo.only(); p.Status != domain.StatusPending {
		t.Fatalf("an unanswered charge leaves the payment pending, got %s", p.Status)
	}
}

type blockingProvider struct{}

func (blockingProvider) Charge(ctx context.Context, _ domain.ChargeRequest) (domain.ChargeResult, error) {
	<-ctx.Done()
	return domain.ChargeResult{}, ctx.Err()
}

func (blockingProvider) Refund(ctx context.Context, _ domain.RefundRequest) (domain.RefundResult, error) {
	<-ctx.Done()
	return domain.RefundResult{}, ctx.Err()
}

func TestPendingPaymentLeftByACrashIsContinued(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	in := request("key-1")
	now := time.Now()
	// The previous worker stored the payment and died before calling the provider.
	id := uuid.New()
	repo.payments[id] = &domain.Payment{
		ID: id, OrderID: in.OrderID, CustomerID: in.CustomerID, IdempotencyKey: in.IdempotencyKey,
		AmountCents: in.AmountCents, Currency: in.Currency, PaymentMethod: in.PaymentMethod,
		Status: domain.StatusPending, CreatedAt: now, UpdatedAt: now,
	}

	mustSucceed(t, newService(repo, prov, 3).Process(context.Background(), in))

	if repo.count() != 1 || repo.only().Status != domain.StatusSucceeded || prov.callCount() != 1 {
		t.Fatalf("the pending payment should have been completed once: %+v", repo.only())
	}
}

func TestSecondPaymentForAPaidOrderIsRefusedWithoutCharging(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc := newService(repo, prov, 3)
	first := request("key-1")
	mustSucceed(t, svc.Process(context.Background(), first))

	second := first
	second.IdempotencyKey = "key-2" // the customer clicked "pay" again with a fresh intent
	mustSucceed(t, svc.Process(context.Background(), second))

	if prov.callCount() != 1 || len(prov.charged) != 1 {
		t.Fatalf("the order must be charged once, provider calls=%d", prov.callCount())
	}
	latest, _ := repo.LatestByOrder(context.Background(), first.OrderID)
	if latest.IdempotencyKey != "key-2" || latest.Status != domain.StatusFailed || latest.FailureCode != domain.CodeOrderAlreadyPaid {
		t.Fatalf("unexpected second payment: %+v", latest)
	}
}

func TestConcurrentRequestsForOneKeyChargeOnce(t *testing.T) {
	repo := newMemRepo()
	prov := newProvider(func(call int, req domain.ChargeRequest) (domain.ChargeResult, error) {
		time.Sleep(10 * time.Millisecond) // widen the race window
		return approve(call, req)
	})
	svc := newService(repo, prov, 3)
	in := request("key-1")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.Process(context.Background(), in); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if repo.count() != 1 {
		t.Fatalf("%d payments were created for one key", repo.count())
	}
	if len(prov.charged) != 1 {
		t.Fatalf("the customer was charged %d times", len(prov.charged))
	}
	if repo.only().Status != domain.StatusSucceeded {
		t.Fatal("payment did not succeed")
	}
}

func TestInvalidRequestsAreRejectedBeforeAnythingHappens(t *testing.T) {
	cases := map[string]func(*application.ProcessInput){
		"no order":          func(i *application.ProcessInput) { i.OrderID = uuid.Nil },
		"no customer":       func(i *application.ProcessInput) { i.CustomerID = uuid.Nil },
		"no key":            func(i *application.ProcessInput) { i.IdempotencyKey = "" },
		"huge key":          func(i *application.ProcessInput) { i.IdempotencyKey = string(make([]byte, 201)) },
		"zero amount":       func(i *application.ProcessInput) { i.AmountCents = 0 },
		"negative amount":   func(i *application.ProcessInput) { i.AmountCents = -5 },
		"other currency":    func(i *application.ProcessInput) { i.Currency = "USD" },
		"no payment method": func(i *application.ProcessInput) { i.PaymentMethod = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			repo, prov := newMemRepo(), newProvider(approve)
			in := request("key-1")
			mutate(&in)

			err := newService(repo, prov, 3).Process(context.Background(), in)

			if !errors.Is(err, domain.ErrInvalidRequest) {
				t.Fatalf("got %v", err)
			}
			if repo.count() != 0 || prov.callCount() != 0 || len(repo.events()) != 0 {
				t.Fatal("an invalid request must leave no trace")
			}
		})
	}
}

func TestQueryServiceHidesOtherCustomersPayments(t *testing.T) {
	repo := newMemRepo()
	in := request("key-1")
	mustSucceed(t, newService(repo, newProvider(approve), 3).Process(context.Background(), in))
	q := application.NewQueryService(repo)

	if p, err := q.LatestByOrderForCustomer(context.Background(), in.OrderID, in.CustomerID); err != nil || p.OrderID != in.OrderID {
		t.Fatalf("owner must see the payment: %v", err)
	}
	_, err := q.LatestByOrderForCustomer(context.Background(), in.OrderID, uuid.New())
	if !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("a stranger must get not-found, got %v", err)
	}
}

// ---------------------------------------------------------------- refunds

func paidOrder(t *testing.T, repo *memRepo, prov *scriptedProvider) (*application.PaymentService, application.ProcessInput) {
	t.Helper()
	svc := newService(repo, prov, 3)
	in := request("key-1")
	mustSucceed(t, svc.Process(context.Background(), in))
	return svc, in
}

func refundEvents(repo *memRepo) []events.PaymentRefundedPayload {
	var out []events.PaymentRefundedPayload
	for _, e := range repo.events() {
		if e.Type == events.PaymentRefunded {
			var p events.PaymentRefundedPayload
			_ = e.DecodePayload(&p)
			out = append(out, p)
		}
	}
	return out
}

func TestRefundGivesTheMoneyBackAndAnnouncesIt(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc, in := paidOrder(t, repo, prov)

	mustSucceed(t, svc.Refund(context.Background(), application.RefundInput{OrderID: in.OrderID, Reason: "payment_timeout"}))

	p := repo.only()
	if p.Status != domain.StatusRefunded || p.RefundRef != "rf_ref_key-1" || p.RefundedAt == nil || p.ProviderRef == "" {
		t.Fatalf("payment = %+v", p)
	}
	got := refundEvents(repo)
	if len(got) != 1 || got[0].PaymentID != p.ID || got[0].OrderID != in.OrderID || got[0].AmountCents != in.AmountCents || got[0].RefundRef != "rf_ref_key-1" {
		t.Fatalf("events = %+v", got)
	}
}

func TestRepeatedRefundRequestsRefundOnceAndAnswerEachTime(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc, in := paidOrder(t, repo, prov)
	for i := 0; i < 3; i++ {
		mustSucceed(t, svc.Refund(context.Background(), application.RefundInput{OrderID: in.OrderID}))
	}
	if prov.refundCalls != 1 || len(prov.refunded) != 1 {
		t.Fatalf("the provider was asked to refund %d times", prov.refundCalls)
	}
	// Order may never have received the first answer, so each request gets one.
	if got := refundEvents(repo); len(got) != 3 {
		t.Fatalf("expected an answer per request, got %d", len(got))
	}
}

func TestRefundWithoutAPaidPaymentIsRefused(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(func(int, domain.ChargeRequest) (domain.ChargeResult, error) {
		return domain.ChargeResult{DeclineCode: domain.CodeCardDeclined}, nil
	})
	svc := newService(repo, prov, 3)
	in := request("key-1")
	mustSucceed(t, svc.Process(context.Background(), in)) // declined

	err := svc.Refund(context.Background(), application.RefundInput{OrderID: in.OrderID})
	if !errors.Is(err, domain.ErrNothingToRefund) {
		t.Fatalf("a declined payment has nothing to refund, err = %v", err)
	}
	if err := svc.Refund(context.Background(), application.RefundInput{OrderID: uuid.New()}); !errors.Is(err, domain.ErrNothingToRefund) {
		t.Fatalf("unknown order: %v", err)
	}
	if err := svc.Refund(context.Background(), application.RefundInput{}); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("no order id: %v", err)
	}
	if prov.refundCalls != 0 {
		t.Fatal("the provider must not be asked to refund money that was never taken")
	}
}

func TestProviderOutageDuringRefundLeavesThePaymentSucceededForARetry(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc, in := paidOrder(t, repo, prov)

	prov.refundErr = errOutage
	if err := svc.Refund(context.Background(), application.RefundInput{OrderID: in.OrderID}); !errors.Is(err, errOutage) {
		t.Fatalf("the error must reach the consumer so the message is retried, got %v", err)
	}
	if p := repo.only(); p.Status != domain.StatusSucceeded || len(refundEvents(repo)) != 0 {
		t.Fatalf("nothing may change while the provider is down: %+v", p)
	}

	prov.refundErr = nil
	mustSucceed(t, svc.Refund(context.Background(), application.RefundInput{OrderID: in.OrderID}))
	if repo.only().Status != domain.StatusRefunded {
		t.Fatal("the retry must refund")
	}
}

func TestPaymentRequestedForARefundedPaymentAnswersWithTheRefund(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc, in := paidOrder(t, repo, prov)
	mustSucceed(t, svc.Refund(context.Background(), application.RefundInput{OrderID: in.OrderID}))
	before := len(repo.events())

	mustSucceed(t, svc.Process(context.Background(), in)) // Order's recovery asks again, same key
	evs := repo.events()
	if len(evs) != before+1 || evs[len(evs)-1].Type != events.PaymentRefunded {
		t.Fatalf("expected PaymentRefunded to be sent again, got %v", evs[before:])
	}
	if prov.callCount() != 1 {
		t.Fatal("no new charge")
	}
}

func TestAnOrderWhosePaymentWasRefundedCanNotBeChargedAgain(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc, in := paidOrder(t, repo, prov)
	mustSucceed(t, svc.Refund(context.Background(), application.RefundInput{OrderID: in.OrderID}))

	again := in
	again.IdempotencyKey = "key-2"
	mustSucceed(t, svc.Process(context.Background(), again))
	if prov.callCount() != 1 {
		t.Fatalf("the provider was called %d times", prov.callCount())
	}
	if latest, _ := repo.LatestByOrder(context.Background(), in.OrderID); latest.FailureCode != domain.CodeOrderAlreadyPaid {
		t.Fatalf("second attempt = %+v", latest)
	}
}

// blindPreCheck lets a second payment through the "already paid?" check, as happens when two
// requests with different keys for one order are checked before either has completed.
type blindPreCheck struct{ *memRepo }

func (blindPreCheck) HasSucceeded(context.Context, uuid.UUID) (bool, error) { return false, nil }

func racingService(repo *memRepo, prov *scriptedProvider) *application.PaymentService {
	return application.NewPaymentService(blindPreCheck{repo}, prov, quietLog(), application.Config{MaxAttempts: 3, ChargeTimeout: time.Second})
}

// The provider charges both; the database accepts only the first success. The second charge must
// be refunded on the spot instead of waiting for a person.
func TestDuplicateChargeThatRacedPastThePreCheckIsRefunded(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc := racingService(repo, prov)
	first := request("key-1")
	mustSucceed(t, svc.Process(context.Background(), first))

	second := first
	second.IdempotencyKey = "key-2"
	mustSucceed(t, svc.Process(context.Background(), second))

	if prov.refundCalls != 1 || !prov.refunded["duplicate:"+repo.byKeyID("key-2").String()] {
		t.Fatalf("the duplicate charge must be refunded, refund calls = %d", prov.refundCalls)
	}
	if len(prov.charged) != 2 {
		t.Fatal("setup: the provider should have charged twice")
	}
	dup := repo.byKeyCopy("key-2")
	if dup.Status != domain.StatusFailed || dup.FailureCode != domain.CodeOrderAlreadyPaid {
		t.Fatalf("the duplicate must end as failed/order_already_paid: %+v", dup)
	}
	if repo.byKeyCopy("key-1").Status != domain.StatusSucceeded {
		t.Fatal("the first payment must stay succeeded")
	}
}

func TestDuplicateChargeWhoseRefundFailsIsRetriedNotForgotten(t *testing.T) {
	repo, prov := newMemRepo(), newProvider(approve)
	svc := racingService(repo, prov)
	first := request("key-1")
	mustSucceed(t, svc.Process(context.Background(), first))
	second := first
	second.IdempotencyKey = "key-2"

	prov.refundErr = errOutage
	if err := svc.Process(context.Background(), second); !errors.Is(err, errOutage) {
		t.Fatalf("the message must be retried, err = %v", err)
	}
	if repo.byKeyCopy("key-2").Status != domain.StatusPending {
		t.Fatal("the duplicate stays pending until the refund went through")
	}
}
