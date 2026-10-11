package httpx

import (
	"github.com/Furkanturan8/rovena/shared/telemetry"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	RequestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
)

// RequestIDMiddleware propagates an incoming X-Request-ID or generates one.
func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals(requestIDKey, id)
		c.Set(RequestIDHeader, id)
		return c.Next()
	}
}

func RequestID(c *fiber.Ctx) string {
	id, _ := c.Locals(requestIDKey).(string)
	return id
}

// AccessLog writes one structured line per request. Bodies and auth headers are never logged.
// Errors are rendered here (via the app's ErrorHandler) so the logged status is the real one.
func AccessLog(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		if err != nil {
			if handlerErr := c.App().ErrorHandler(c, err); handlerErr != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}
		attrs := []any{
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
			"request_id", RequestID(c),
		}
		if id := telemetry.TraceID(c.UserContext()); id != "" {
			attrs = append(attrs, "trace_id", id)
		}
		log.Info("http request", attrs...)
		return nil
	}
}
