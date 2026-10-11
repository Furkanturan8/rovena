package provider_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/infrastructure/provider"
)

func charge(t *testing.T, f *provider.Fake, key, method string, amount int64) (domain.ChargeResult, error) {
	t.Helper()
	return f.Charge(context.Background(), domain.ChargeRequest{
		IdempotencyKey: key, AmountCents: amount, Currency: "TRY", PaymentMethod: method,
	})
}

func TestOutcomeFollowsTheTestMethod(t *testing.T) {
	cases := []struct {
		method   string
		amount   int64
		approved bool
		code     string
	}{
		{provider.MethodApproved, 1000, true, ""},
		{provider.MethodDeclined, 1000, false, domain.CodeCardDeclined},
		{provider.MethodInsufficientFunds, 1000, false, domain.CodeInsufficientFunds},
		{provider.MethodExpired, 1000, false, domain.CodeExpiredCard},
		{"tok_unknown", 1000, false, domain.CodeInvalidMethod},
		{provider.MethodApproved, provider.MaxAmountCents, true, ""},
		{provider.MethodApproved, provider.MaxAmountCents + 1, false, domain.CodeAmountLimit},
	}
	for _, c := range cases {
		t.Run(c.method, func(t *testing.T) {
			res, err := charge(t, provider.NewFake(0), "k", c.method, c.amount)
			if err != nil {
				t.Fatal(err)
			}
			if res.Approved != c.approved || res.DeclineCode != c.code {
				t.Fatalf("got %+v", res)
			}
			if res.Approved && res.Reference == "" {
				t.Fatal("an approved charge needs a provider reference")
			}
			if !res.Approved && res.DeclineMessage == "" {
				t.Fatal("a decline needs a message")
			}
		})
	}
}

func TestSameKeyGetsTheSameAnswer(t *testing.T) {
	f := provider.NewFake(0)
	first, _ := charge(t, f, "k", provider.MethodApproved, 1000)
	// A repeat with a different amount still returns the first result: the key, not the
	// body, identifies the charge, as with real providers.
	again, _ := charge(t, f, "k", provider.MethodApproved, 2000)
	if first != again {
		t.Fatalf("%+v vs %+v", first, again)
	}
	other, _ := charge(t, f, "k2", provider.MethodApproved, 1000)
	if other.Reference == first.Reference {
		t.Fatal("different keys are different charges")
	}
}

func TestProviderErrorMethodNeverAnswers(t *testing.T) {
	f := provider.NewFake(0)
	for i := 0; i < 5; i++ {
		if _, err := charge(t, f, "k", provider.MethodProviderError, 1000); !errors.Is(err, provider.ErrUnavailable) {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestFlakyMethodFailsTwiceThenApproves(t *testing.T) {
	f := provider.NewFake(0)
	for i := 0; i < 2; i++ {
		if _, err := charge(t, f, "k", provider.MethodFlaky, 1000); err == nil {
			t.Fatalf("call %d should fail", i+1)
		}
	}
	res, err := charge(t, f, "k", provider.MethodFlaky, 1000)
	if err != nil || !res.Approved {
		t.Fatalf("third call: %+v, %v", res, err)
	}
	// Another payment starts its own count.
	if _, err := charge(t, f, "other", provider.MethodFlaky, 1000); err == nil {
		t.Fatal("a new key starts failing again")
	}
}

func TestSlowMethodEndsOnlyWithTheCallersTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := provider.NewFake(0).Charge(ctx, domain.ChargeRequest{
		IdempotencyKey: "k", AmountCents: 1000, Currency: "TRY", PaymentMethod: provider.MethodSlow,
	})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("err=%v after %s", err, time.Since(start))
	}
}

func TestLatencyIsAppliedAndCancellable(t *testing.T) {
	f := provider.NewFake(50 * time.Millisecond)
	start := time.Now()
	if _, err := charge(t, f, "k", provider.MethodApproved, 1000); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("latency was not applied")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Charge(ctx, domain.ChargeRequest{IdempotencyKey: "k2", PaymentMethod: provider.MethodApproved}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestConcurrentCallsAreSafe(t *testing.T) {
	f := provider.NewFake(0)
	var wg sync.WaitGroup
	refs := make(chan string, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := charge(t, f, "same", provider.MethodApproved, 1000)
			refs <- res.Reference
		}()
	}
	wg.Wait()
	close(refs)
	first := ""
	for r := range refs {
		if first == "" {
			first = r
		}
		if r != first {
			t.Fatalf("got two references for one key: %q and %q", first, r)
		}
	}
}

func TestRefundIsIdempotentPerKey(t *testing.T) {
	f := provider.NewFake(0)
	req := domain.RefundRequest{IdempotencyKey: "refund:1", ChargeRef: "fake_k", AmountCents: 1000, Currency: "TRY"}
	first, err := f.Refund(context.Background(), req)
	if err != nil || first.Reference == "" {
		t.Fatalf("%+v, %v", first, err)
	}
	again, _ := f.Refund(context.Background(), req)
	if again != first {
		t.Fatalf("a repeated key must return the first answer: %+v vs %+v", again, first)
	}
	other, _ := f.Refund(context.Background(), domain.RefundRequest{IdempotencyKey: "refund:2", ChargeRef: "fake_other"})
	if other.Reference == first.Reference {
		t.Fatal("refunds of different charges need different references")
	}
}

func TestRefundHonoursTheCallersDeadline(t *testing.T) {
	f := provider.NewFake(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := f.Refund(ctx, domain.RefundRequest{IdempotencyKey: "k"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}
