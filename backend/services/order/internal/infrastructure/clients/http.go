// Package clients holds the REST clients Order uses to call Catalog and Inventory.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const internalKeyHeader = "X-Internal-Key"

// maxResponseBytes bounds what is read from a peer; none of the answers come close.
const maxResponseBytes = 1 << 20

var clientDuration = telemetry.DurationHistogram("http.client.request.duration", "Duration of calls to other services, by peer, route and outcome")

type base struct {
	peer string // "catalog", "inventory": names the callee in spans and metrics
	url  string
	key  string
	hc   *http.Client
}

func newBase(peer, url, internalKey string, timeout time.Duration) base {
	return base{peer: peer, url: url, key: internalKey, hc: &http.Client{Timeout: timeout}}
}

// do sends a JSON request and returns the status and body. A transport error (timeout,
// connection refused) is returned as an error; any HTTP status is returned as is.
func (b base) do(ctx context.Context, method, path string, body any) (status int, respBody []byte, err error) {
	// Spans and metrics name the callee and method only, never the URL: ids in paths would
	// create a new time series for every order.
	ctx, span := telemetry.Tracer().Start(ctx, method+" "+b.peer, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("peer.service", b.peer), attribute.String("http.request.method", method)))
	start := time.Now()
	defer func() {
		outcome := "error"
		if err == nil {
			outcome = strconv.Itoa(status)
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			if status >= 500 {
				span.SetStatus(codes.Error, outcome)
			}
		} else {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
		clientDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
			attribute.String("peer", b.peer), attribute.String("method", method), attribute.String("outcome", outcome)))
	}()

	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.url+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set(internalKeyHeader, b.key)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if id := messaging.RequestID(ctx); id != "" {
		req.Header.Set("X-Request-ID", id)
	}
	resp, err := b.hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, raw, nil
}

func unexpected(service string, status int, body []byte) error {
	if len(body) > 200 {
		body = body[:200]
	}
	return fmt.Errorf("%s answered %d: %s", service, status, body)
}
