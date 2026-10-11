package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestLogLinesCarryTheTraceOfTheirContext(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(traceHandler{slog.NewJSONHandler(&buf, nil)}).With("service", "x")

	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID}))

	log.InfoContext(ctx, "with trace")
	log.Info("without trace")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if !strings.Contains(lines[0], `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`) || !strings.Contains(lines[0], `"span_id":"00f067aa0ba902b7"`) {
		t.Errorf("trace ids missing: %s", lines[0])
	}
	if !strings.Contains(lines[0], `"service":"x"`) {
		t.Errorf("With() attributes lost: %s", lines[0])
	}
	if strings.Contains(lines[1], "trace_id") {
		t.Errorf("a context without a span must not invent ids: %s", lines[1])
	}
}
