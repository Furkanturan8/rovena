package http

import (
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
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
	Readiness      map[string]httpx.ReadinessCheck
}

type Services struct {
	Carts   *application.CartService
	Orders  *application.OrderService
	Queries *application.QueryService
}

func NewApp(opts Options, svc Services) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "rovena-order",
		DisableStartupMessage: true,
		BodyLimit:             maxBodyBytes,
		ReadTimeout:           10 * time.Second,
		// Checkout calls Inventory before it answers, so writes get more room than elsewhere.
		WriteTimeout:            30 * time.Second,
		IdleTimeout:             60 * time.Second,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          opts.TrustedProxies,
		ProxyHeader:             fiber.HeaderXForwardedFor,
		ErrorHandler:            httpx.NewErrorHandler(opts.Logger, MapDomainError),
	})

	app.Use(httpx.RequestIDMiddleware())
	app.Use(httpx.Tracing())
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(httpx.AccessLog(opts.Logger))
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     opts.AllowedOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Content-Type, Authorization, X-Request-ID, Idempotency-Key",
		AllowCredentials: true,
	}))

	httpx.RegisterHealth(app, opts.Readiness)
	httpx.RegisterMetrics(app) // for Prometheus; the gateway does not route it

	h := NewOrderHandler(svc.Carts, svc.Orders, svc.Queries)
	v1 := app.Group("/api/v1")

	// Everything here belongs to the signed-in customer and only to them. There is no
	// customer id in any path: it always comes from the token.
	authed := auth.Middleware(opts.Verifier)
	v1.Get("/cart", authed, h.GetCart)
	v1.Delete("/cart", authed, h.ClearCart)
	v1.Put("/cart/items/:variantId", authed, h.SetCartItem)
	v1.Delete("/cart/items/:variantId", authed, h.RemoveCartItem)
	v1.Post("/checkout", authed, h.Checkout)
	v1.Get("/orders", authed, h.MyOrders)
	v1.Get("/orders/:id", authed, h.MyOrder)
	v1.Post("/orders/:id/cancel", authed, h.Cancel)

	admin := v1.Group("/admin", auth.Middleware(opts.Verifier), auth.RequireRole(auth.RoleAdmin))
	admin.Get("/orders", h.AdminList)
	admin.Get("/orders/:id", h.AdminGet)

	return app
}
