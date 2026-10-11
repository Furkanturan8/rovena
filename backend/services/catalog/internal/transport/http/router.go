package http

import (
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const maxBodyBytes = 256 * 1024

type Options struct {
	Logger         *slog.Logger
	AllowedOrigins string
	TrustedProxies []string
	Verifier       *auth.Verifier
	InternalAPIKey string
	Readiness      map[string]httpx.ReadinessCheck
}

type Services struct {
	Categories *application.CategoryService
	Products   *application.ProductService
}

func NewApp(opts Options, svc Services) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "rovena-catalog",
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
	app.Use(httpx.Tracing())
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(httpx.AccessLog(opts.Logger))
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     opts.AllowedOrigins,
		AllowMethods:     "GET,POST,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Content-Type, Authorization, X-Request-ID",
		AllowCredentials: true,
	}))

	httpx.RegisterHealth(app, opts.Readiness)
	httpx.RegisterMetrics(app) // for Prometheus; the gateway does not route it

	categories := NewCategoryHandler(svc.Categories)
	products := NewProductHandler(svc.Products)

	v1 := app.Group("/api/v1")

	// Public, read-only storefront API.
	v1.Get("/categories", categories.List)
	v1.Get("/categories/:slug", categories.GetBySlug)
	v1.Get("/products", products.List)
	v1.Get("/products/:idOrSlug", products.Get)

	// Service-to-service. Not routed by the gateway, guarded by the internal key.
	internal := v1.Group("/internal", RequireInternalKey(opts.InternalAPIKey))
	internal.Post("/variants/lookup", products.LookupVariants)

	// Admin API. Tokens are issued by Identity and verified locally.
	admin := v1.Group("/admin", auth.Middleware(opts.Verifier), auth.RequireRole(auth.RoleAdmin))

	admin.Post("/categories", categories.Create)
	admin.Patch("/categories/:id", categories.Update)
	admin.Delete("/categories/:id", categories.Delete)

	admin.Get("/products", products.AdminList)
	admin.Post("/products", products.Create)
	admin.Get("/products/:id", products.AdminGet)
	admin.Patch("/products/:id", products.Update)
	admin.Delete("/products/:id", products.Archive)
	admin.Post("/products/:id/variants", products.AddVariant)
	admin.Patch("/products/:id/variants/:variantId", products.UpdateVariant)

	return app
}
