// Package telemetry sets up OpenTelemetry for a service: trace propagation (W3C
// traceparent), spans, and metrics exposed in Prometheus format.
//
// Without configuration everything still works and costs almost nothing: spans are created
// (so trace ids exist in logs and are passed on to other services and through the outbox)
// but nothing is exported. Set OTEL_EXPORTER_OTLP_ENDPOINT (for example http://jaeger:4318)
// to send traces to a collector. Metrics are always collected and served at /metrics.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const instrumentation = "github.com/Furkanturan8/rovena/shared"

var (
	metricsHandler atomic.Pointer[http.Handler]
	setupMu        sync.Mutex
	isSetUp        bool
)

// Setup installs the global tracer provider, meter provider and propagator for service.
// Call the returned function on shutdown so buffered spans are flushed.
//
// It takes effect once per process: OpenTelemetry's global providers can be replaced only
// before the first instrument is created, so a second call (several tests in one binary) does
// nothing and returns a no-op shutdown.
func Setup(ctx context.Context, service string, log *slog.Logger) (shutdown func(context.Context) error, err error) {
	setupMu.Lock()
	defer setupMu.Unlock()
	if isSetUp {
		return func(context.Context) error { return nil }, nil
	}
	if log == nil {
		log = slog.Default()
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", service)),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, err
	}

	// Traces. The exporter is only created when an endpoint is configured.
	tpOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	exporting := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
	if exporting {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Metrics, scraped by Prometheus. Go runtime and process metrics come along.
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reader, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	metricsHandler.Store(&h)

	isSetUp = true
	log.InfoContext(ctx, "telemetry ready", "service", service, "trace_export", exporting)
	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}

// MetricsHandler serves the Prometheus text format. Before Setup it answers 503.
func MetricsHandler() http.Handler {
	if h := metricsHandler.Load(); h != nil {
		return *h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "telemetry is not set up", http.StatusServiceUnavailable)
	})
}

func Tracer() trace.Tracer { return otel.Tracer(instrumentation) }

func Meter() metric.Meter { return otel.Meter(instrumentation) }

// Traceparent returns the W3C traceparent of the span in ctx, or "" when there is none.
func Traceparent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

// WithTraceparent returns ctx continuing the trace named by a traceparent value; an empty
// or invalid value leaves ctx unchanged.
func WithTraceparent(ctx context.Context, traceparent string) context.Context {
	if traceparent == "" {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier{"traceparent": traceparent})
}

// TraceID returns the trace id of the span in ctx, or "" when there is none.
func TraceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID().String()
	}
	return ""
}

// Counter creates (or returns) a counter; creating an instrument can only fail on an invalid
// name, which is a programming error that must not take a service down.
func Counter(name, description string) metric.Int64Counter {
	c, err := Meter().Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		slog.Error("telemetry: counter", "name", name, "error", err)
		c, _ = noop.NewMeterProvider().Meter("").Int64Counter(name)
	}
	return c
}

// DurationHistogram creates a histogram of seconds with buckets suited to request latencies.
func DurationHistogram(name, description string) metric.Float64Histogram {
	h, err := Meter().Float64Histogram(name, metric.WithUnit("s"), metric.WithDescription(description),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30))
	if err != nil {
		slog.Error("telemetry: histogram", "name", name, "error", err)
		h, _ = noop.NewMeterProvider().Meter("").Float64Histogram(name)
	}
	return h
}
