package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Envelope is the stable wrapper around every event on the bus. Consumers read the
// metadata without knowing the payload shape, and Version lets a payload evolve
// without breaking consumers that still expect the old one.
type Envelope struct {
	// ID identifies this event for its whole life. Redeliveries carry the same ID,
	// which is what consumers deduplicate on.
	ID         uuid.UUID `json:"event_id"`
	Type       string    `json:"type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	// CorrelationID ties together every event of one business flow (one checkout).
	CorrelationID string `json:"correlation_id"`
	// RequestID is the X-Request-ID of the HTTP request that started the flow, so a
	// log search can cross from the API call into the asynchronous part.
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEnvelope wraps payload as an event of the given type and version. Correlation and
// request IDs come from ctx (see WithCorrelationID); a missing correlation ID starts a
// new flow.
func NewEnvelope(ctx context.Context, eventType string, version int, payload any) (Envelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal %s payload: %w", eventType, err)
	}
	correlationID := CorrelationID(ctx)
	if correlationID == "" {
		correlationID = uuid.NewString()
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ID:            id,
		Type:          eventType,
		Version:       version,
		OccurredAt:    time.Now().UTC(),
		CorrelationID: correlationID,
		RequestID:     RequestID(ctx),
		Payload:       body,
	}, nil
}

// DecodePayload unmarshals the payload into v.
func (e Envelope) DecodePayload(v any) error {
	if err := json.Unmarshal(e.Payload, v); err != nil {
		return fmt.Errorf("decode %s payload: %w", e.Type, err)
	}
	return nil
}

// Validate reports whether the envelope carries the metadata every consumer relies on.
func (e Envelope) Validate() error {
	switch {
	case e.ID == uuid.Nil:
		return errors.New("envelope: missing event_id")
	case e.Type == "":
		return errors.New("envelope: missing type")
	case e.Version < 1:
		return errors.New("envelope: version must be at least 1")
	case e.CorrelationID == "":
		return errors.New("envelope: missing correlation_id")
	case len(e.Payload) == 0:
		return errors.New("envelope: missing payload")
	}
	return nil
}

type ctxKey int

const (
	correlationKey ctxKey = iota
	requestKey
)

// WithCorrelationID returns a context whose events join the given flow.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey, id)
}

func CorrelationID(ctx context.Context) string {
	id, _ := ctx.Value(correlationKey).(string)
	return id
}

// WithRequestID returns a context that stamps events with the originating HTTP request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestKey).(string)
	return id
}

// ContextFor returns ctx carrying the envelope's correlation and request IDs, so
// anything a handler publishes in reaction stays in the same flow.
func ContextFor(ctx context.Context, e Envelope) context.Context {
	ctx = WithCorrelationID(ctx, e.CorrelationID)
	return WithRequestID(ctx, e.RequestID)
}
