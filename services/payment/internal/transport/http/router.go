package http

import (
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const maxBodyBytes = 16 * 1024

type Options struct {
	Logger         *slog.Logger
	AllowedOrigins string
	TrustedProxies []string
	Verifier       *auth.Verifier
	InternalAPIKey string
	Readiness      map[string]httpx.ReadinessCheck
}

type Services struct {
	Payments *application.QueryService
}

func NewApp(opts Options, svc Services) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "rovena-payment",
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
		AllowMethods:     "GET,OPTIONS",
		AllowHeaders:     "Content-Type, Authorization, X-Request-ID",
		AllowCredentials: true,
	}))

	httpx.RegisterHealth(app, opts.Readiness)

	payments := NewPaymentHandler(svc.Payments)
	v1 := app.Group("/api/v1")

	// A customer reads the state of their own payment. No endpoint creates or changes
	// a payment: that happens only through PaymentRequested events.
	v1.Get("/payments/by-order/:orderId", auth.Middleware(opts.Verifier), payments.MyByOrder)

	// Service-to-service. Not routed by the gateway, guarded by the internal key.
	internal := v1.Group("/internal", RequireInternalKey(opts.InternalAPIKey))
	internal.Get("/payments/by-order/:orderId", payments.InternalByOrder)

	admin := v1.Group("/admin", auth.Middleware(opts.Verifier), auth.RequireRole(auth.RoleAdmin))
	admin.Get("/payments", payments.AdminList)
	admin.Get("/payments/:id", payments.AdminGet)

	return app
}
