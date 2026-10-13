package http

import (
	"log/slog"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const maxBodyBytes = 64 * 1024

type Options struct {
	Logger         *slog.Logger
	AllowedOrigins string
	TrustedProxies []string
	Verifier       *auth.Verifier
	Readiness      map[string]httpx.ReadinessCheck
}

type Services struct {
	Auth      *application.AuthService
	Users     *application.UserService
	Addresses *application.AddressService
}

func NewApp(opts Options, svc Services) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "rovena-identity",
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
		AllowMethods:     strings.Join([]string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}, ","),
		AllowHeaders:     "Content-Type, Authorization, X-Request-ID",
		AllowCredentials: true,
	}))

	httpx.RegisterHealth(app, opts.Readiness)
	httpx.RegisterMetrics(app) // for Prometheus; the gateway does not route it

	authHandler := NewAuthHandler(svc.Auth)
	userHandler := NewUserHandler(svc.Users)
	accountHandler := NewAccountHandler(svc.Auth, svc.Addresses)
	authenticated := auth.Middleware(opts.Verifier)
	adminOnly := auth.RequireRole(auth.RoleAdmin)

	v1 := app.Group("/api/v1")

	authGroup := v1.Group("/auth", limiter.New(limiter.Config{
		Max:        20,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Too many requests, try again later")
		},
	}))
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/refresh", authHandler.Refresh)
	authGroup.Post("/logout", authHandler.Logout)

	users := v1.Group("/users", authenticated)
	users.Get("/me", userHandler.Me)
	users.Patch("/me", userHandler.UpdateMe)
	sensitive := limiter.New(limiter.Config{
		Max:          10,
		Expiration:   time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Too many requests, try again later")
		},
	})
	users.Post("/me/password", sensitive, accountHandler.ChangePassword)
	users.Post("/me/email", sensitive, accountHandler.ChangeEmail)
	users.Get("/me/addresses", accountHandler.ListAddresses)
	users.Post("/me/addresses", accountHandler.CreateAddress)
	users.Patch("/me/addresses/:id", accountHandler.UpdateAddress)
	users.Delete("/me/addresses/:id", accountHandler.DeleteAddress)
	users.Post("/me/addresses/:id/default", accountHandler.SetDefaultAddress)
	users.Get("/", adminOnly, userHandler.List)
	users.Get("/:id", adminOnly, userHandler.GetByID)
	users.Patch("/:id/role", adminOnly, userHandler.ChangeRole)

	return app
}
