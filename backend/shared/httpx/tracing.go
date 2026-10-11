package httpx

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/shared/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// observability endpoints are not traced or counted: scraping them every few seconds would
// drown the interesting requests.
var untraced = map[string]bool{"/healthz": true, "/readyz": true, "/metrics": true}

type headerCarrier struct{ c *fiber.Ctx }

func (h headerCarrier) Get(key string) string { return h.c.Get(key) }
func (h headerCarrier) Set(key, v string)     { h.c.Set(key, v) }
func (h headerCarrier) Keys() []string {
	var keys []string
	h.c.Request().Header.VisitAll(func(k, _ []byte) { keys = append(keys, string(k)) })
	return keys
}

// Tracing continues the caller's trace (W3C traceparent header, as sent by another service or
// by a client), starts a server span, answers with X-Trace-ID, and records request count and
// duration labelled by method, route template and status. Place it first, before AccessLog and
// recovery, so panics and rendered errors are seen with their final status.
func Tracing() fiber.Handler {
	tracer := telemetry.Tracer()
	duration := telemetry.DurationHistogram("http.server.request.duration", "Duration of HTTP requests served")

	return func(c *fiber.Ctx) error {
		if untraced[c.Path()] {
			return c.Next()
		}
		ctx := otel.GetTextMapPropagator().Extract(c.UserContext(), headerCarrier{c})
		// Fiber hands out strings that point into buffers it reuses for the next request; the span and
		// the metric outlive this call, so they get their own copies.
		method := strings.Clone(c.Method())
		ctx, span := tracer.Start(ctx, method+" "+strings.Clone(c.Path()), trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		c.SetUserContext(ctx)
		c.Set("X-Trace-ID", span.SpanContext().TraceID().String())

		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil && status < fiber.StatusBadRequest {
			// Not rendered yet (no error handler ran inside us): derive it the way Fiber will.
			var fe *fiber.Error
			if errors.As(err, &fe) {
				status = fe.Code
			} else {
				status = fiber.StatusInternalServerError // for example a panic that recovery turned into an error
			}
		}

		route := "unmatched"
		if r := c.Route(); r != nil && (r.Path != "/" || status != fiber.StatusNotFound) {
			route = strings.Clone(r.Path)
		}
		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		}
		span.SetName(method + " " + route)
		span.SetAttributes(attrs...)
		if status >= fiber.StatusInternalServerError {
			span.SetStatus(codes.Error, strconv.Itoa(status))
		}
		duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		return err
	}
}

// RegisterMetrics serves the Prometheus metrics at /metrics. It is not routed by the gateway.
func RegisterMetrics(app *fiber.App) {
	app.Get("/metrics", func(c *fiber.Ctx) error {
		return adaptor.HTTPHandler(telemetry.MetricsHandler())(c)
	})
}
