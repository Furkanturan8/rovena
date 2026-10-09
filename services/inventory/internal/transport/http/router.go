package http

import (
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const maxBodyBytes = 64 * 1024

type Options struct {
	Logger         *slog.Logger
	AllowedOrigins string
	TrustedProxies []string
	Verifier       *auth.Verifier
	InternalAPIKey string
	Readiness      map[string]httpx.ReadinessCheck
}

type Services struct {
	Stock        *application.StockService
	Reservations *application.ReservationService
}

func NewApp(opts Options, svc Services) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "rovena-inventory",
		DisableStartupMessage:   true,
		BodyLimit:               maxBodyBytes,
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		IdleTimeout:             60 * time.Second,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          opts.TrustedProxies,
		ProxyHeader:             fiber.HeaderXForwardedFor,
		ErrorHandler:            httpx.NewErrorHandler(opts.Logger, MapDomainError),
	})

	app.Use(httpx.RequestIDMiddleware())
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(httpx.AccessLog(opts.Logger))
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     opts.AllowedOrigins,
		AllowMethods:     "GET,POST,OPTIONS",
		AllowHeaders:     "Content-Type, Authorization, X-Request-ID",
		AllowCredentials: true,
	}))

	httpx.RegisterHealth(app, opts.Readiness)

	stock := NewStockHandler(svc.Stock)
	reservations := NewReservationHandler(svc.Reservations)

	v1 := app.Group("/api/v1")

	// Public, read-only: the product page asks whether its variants can be bought.
	v1.Get("/stock", stock.Availability)

	// Service-to-service. Not routed by the gateway, guarded by the internal key.
	internal := v1.Group("/internal", RequireInternalKey(opts.InternalAPIKey))
	internal.Post("/reservations", reservations.Create)
	internal.Get("/reservations/:id", reservations.Get)
	internal.Get("/reservations/by-order/:orderId", reservations.GetByOrder)
	internal.Post("/reservations/:id/commit", reservations.Commit)
	internal.Post("/reservations/:id/release", reservations.Release)

	// Warehouse management. Tokens are issued by Identity and verified locally.
	admin := v1.Group("/admin", auth.Middleware(opts.Verifier), auth.RequireRole(auth.RoleAdmin))
	admin.Get("/stock", stock.List)
	admin.Post("/stock", stock.Create)
	admin.Get("/stock/:variantId", stock.Get)
	admin.Post("/stock/:variantId/receipts", stock.Receive)
	admin.Post("/stock/:variantId/adjustments", stock.Adjust)
	admin.Get("/stock/:variantId/movements", stock.Movements)

	return app
}
