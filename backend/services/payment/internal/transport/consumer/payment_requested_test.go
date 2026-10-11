package consumer_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/services/payment/internal/transport/consumer"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
)

// These cases are all decided before the repository or provider is touched, so the
// service can be built without them. Behaviour that needs a database is covered by
// the integration tests.
func newHandler() *consumer.PaymentRequestedHandler {
	svc := application.NewPaymentService(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)),
		application.Config{MaxAttempts: 3, ChargeTimeout: time.Second})
	return consumer.NewPaymentRequestedHandler(svc)
}

func envelope(t *testing.T, version int, payload any) messaging.Envelope {
	t.Helper()
	env, err := messaging.NewEnvelope(context.Background(), events.PaymentRequested, version, payload)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestUnreadableOrUnsupportedMessagesAreNotRetried(t *testing.T) {
	h := newHandler()
	valid := events.PaymentRequestedPayload{
		OrderID: uuid.New(), CustomerID: uuid.New(), IdempotencyKey: "k",
		AmountCents: 1000, Currency: "TRY", PaymentMethod: "tok_visa",
	}

	cases := map[string]messaging.Envelope{
		"future version": envelope(t, 2, valid),
		"payload of wrong type": func() messaging.Envelope {
			e := envelope(t, 1, valid)
			e.Payload = json.RawMessage(`"just a string"`)
			return e
		}(),
		"invalid request": envelope(t, 1, events.PaymentRequestedPayload{IdempotencyKey: "k"}),
		"bad currency": envelope(t, 1, events.PaymentRequestedPayload{
			OrderID: uuid.New(), CustomerID: uuid.New(), IdempotencyKey: "k",
			AmountCents: 1000, Currency: "USD", PaymentMethod: "tok_visa",
		}),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := h.Handle(context.Background(), env)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !messaging.IsPermanent(err) {
				t.Fatalf("%v should be a permanent error so the message goes to the DLQ", err)
			}
		})
	}
}

func TestSubscriptionLeavesRoomForInfrastructureRetries(t *testing.T) {
	sub := consumer.Subscription(3, 5*time.Second)
	if sub.Queue != "payment.requests" || len(sub.Types) != 1 || sub.Types[0] != events.PaymentRequested {
		t.Fatalf("unexpected subscription: %+v", sub)
	}
	if sub.MaxRetries <= 3 || sub.RetryDelay != 5*time.Second {
		t.Fatalf("the consumer needs more redeliveries than provider attempts: %+v", sub)
	}
}
