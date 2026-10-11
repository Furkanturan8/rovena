package telemetry

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

var setupOnce sync.Once

// Setup installs process-wide providers, so tests share one.
func setupForTest(t *testing.T) {
	t.Helper()
	setupOnce.Do(func() {
		if _, err := Setup(context.Background(), "telemetry-test", nil); err != nil {
			t.Fatal(err)
		}
	})
}

func TestTraceparentSurvivesAHopThroughAString(t *testing.T) {
	setupForTest(t)
	ctx, span := Tracer().Start(context.Background(), "origin")
	defer span.End()

	tp := Traceparent(ctx)
	if tp == "" || !strings.HasPrefix(tp, "00-"+TraceID(ctx)+"-") {
		t.Fatalf("traceparent %q does not name trace %q", tp, TraceID(ctx))
	}
	// Another process (or the relay, hours later) continues the same trace from the string alone.
	continued, child := Tracer().Start(WithTraceparent(context.Background(), tp), "continuation")
	defer child.End()
	if TraceID(continued) != TraceID(ctx) {
		t.Fatalf("trace id changed: %s -> %s", TraceID(ctx), TraceID(continued))
	}
	if child.SpanContext().SpanID() == span.SpanContext().SpanID() {
		t.Fatal("the continuation must be a new span")
	}
}

func TestNoSpanMeansNoTraceparent(t *testing.T) {
	setupForTest(t)
	if tp := Traceparent(context.Background()); tp != "" {
		t.Fatalf("got %q", tp)
	}
	if id := TraceID(context.Background()); id != "" {
		t.Fatalf("got %q", id)
	}
	// Garbage must not break anything or start a bogus trace.
	ctx := WithTraceparent(context.Background(), "not-a-traceparent")
	if TraceID(ctx) != "" {
		t.Fatal("an invalid traceparent must be ignored")
	}
}

func TestMetricsAreServedInPrometheusFormat(t *testing.T) {
	setupForTest(t)
	Counter("telemetry.test.events", "test counter").Add(context.Background(), 3)

	rec := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != 200 || !strings.Contains(string(body), "telemetry_test_events_total") || !strings.Contains(string(body), "go_goroutines") {
		t.Fatalf("status %d, body:\n%.600s", rec.Code, body)
	}
}
