package httpx

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ReadinessCheck func(ctx context.Context) error

// RegisterHealth adds /healthz (liveness) and /readyz (dependency readiness).
func RegisterHealth(app *fiber.App, checks map[string]ReadinessCheck) {
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/readyz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		results := make(map[string]string, len(checks))
		ready := true
		for name, check := range checks {
			if err := check(ctx); err != nil {
				results[name] = "unavailable"
				ready = false
				continue
			}
			results[name] = "ok"
		}
		status := fiber.StatusOK
		if !ready {
			status = fiber.StatusServiceUnavailable
		}
		return c.Status(status).JSON(fiber.Map{"ready": ready, "checks": results})
	})
}
