package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type samplePayload struct {
	OrderID string `json:"order_id"`
	Amount  int    `json:"amount"`
}

func TestNewEnvelopeCarriesContextIDs(t *testing.T) {
	ctx := WithRequestID(WithCorrelationID(context.Background(), "flow-1"), "req-9")

	env, err := NewEnvelope(ctx, "PaymentRequested", 1, samplePayload{OrderID: "o1", Amount: 4200})
	if err != nil {
		t.Fatal(err)
	}
	if env.CorrelationID != "flow-1" || env.RequestID != "req-9" {
		t.Fatalf("ids not carried: %+v", env)
	}
	if env.ID == uuid.Nil || env.OccurredAt.IsZero() || env.Version != 1 || env.Type != "PaymentRequested" {
		t.Fatalf("metadata incomplete: %+v", env)
	}
	if err := env.Validate(); err != nil {
		t.Fatal(err)
	}

	var got samplePayload
	if err := env.DecodePayload(&got); err != nil || got.OrderID != "o1" || got.Amount != 4200 {
		t.Fatalf("payload round trip: %+v, %v", got, err)
	}
}

func TestNewEnvelopeStartsNewFlowWithoutCorrelationID(t *testing.T) {
	a, _ := NewEnvelope(context.Background(), "T", 1, struct{}{})
	b, _ := NewEnvelope(context.Background(), "T", 1, struct{}{})
	if a.CorrelationID == "" || a.CorrelationID == b.CorrelationID {
		t.Fatalf("expected distinct generated correlation ids, got %q and %q", a.CorrelationID, b.CorrelationID)
	}
	if a.ID == b.ID {
		t.Fatal("event ids must be unique")
	}
}

func TestEnvelopeSurvivesJSONRoundTrip(t *testing.T) {
	env, _ := NewEnvelope(WithCorrelationID(context.Background(), "c"), "T", 2, samplePayload{OrderID: "o"})
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var back Envelope
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.ID != env.ID || back.Type != env.Type || back.Version != 2 || back.CorrelationID != "c" || !back.OccurredAt.Equal(env.OccurredAt) {
		t.Fatalf("round trip changed envelope: %+v vs %+v", back, env)
	}
}

func TestValidateRejectsIncompleteEnvelopes(t *testing.T) {
	good, _ := NewEnvelope(context.Background(), "T", 1, struct{}{})
	cases := map[string]func(e *Envelope){
		"no id":             func(e *Envelope) { e.ID = uuid.Nil },
		"no type":           func(e *Envelope) { e.Type = "" },
		"version zero":      func(e *Envelope) { e.Version = 0 },
		"no correlation id": func(e *Envelope) { e.CorrelationID = "" },
		"no payload":        func(e *Envelope) { e.Payload = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := good
			mutate(&e)
			if e.Validate() == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestContextForRestoresIDs(t *testing.T) {
	env, _ := NewEnvelope(WithRequestID(WithCorrelationID(context.Background(), "c"), "r"), "T", 1, struct{}{})
	ctx := ContextFor(context.Background(), env)
	if CorrelationID(ctx) != "c" || RequestID(ctx) != "r" {
		t.Fatalf("got %q / %q", CorrelationID(ctx), RequestID(ctx))
	}
}

func TestPermanentWrapsAndUnwraps(t *testing.T) {
	base := errors.New("bad payload")
	err := Permanent(base)
	if !IsPermanent(err) || !errors.Is(err, base) {
		t.Fatal("Permanent must be detectable and keep the cause")
	}
	if IsPermanent(base) {
		t.Fatal("a plain error is not permanent")
	}
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must stay nil")
	}
}

func TestSubscriptionValidation(t *testing.T) {
	ok := Subscription{Queue: "q", Types: []string{"T"}, MaxRetries: 3, RetryDelay: 1}
	if err := ok.validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Subscription{
		{Types: []string{"T"}},
		{Queue: "q"},
		{Queue: "q", Types: []string{"T"}, MaxRetries: -1},
		{Queue: "q", Types: []string{"T"}, MaxRetries: 2},
	}
	for i, s := range bad {
		if s.validate() == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
}
