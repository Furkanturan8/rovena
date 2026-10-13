package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/security"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository/postgres"
	transport "github.com/Furkanturan8/rovena/services/identity/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/logging"
	"github.com/Furkanturan8/rovena/shared/telemetry"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "identity:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateServer(); err != nil {
		return err
	}
	log := logging.New("identity", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := telemetry.Setup(ctx, "identity", log)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(flushCtx)
	}()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	db = telemetry.InstrumentBun(db) // one span per query, inside the trace of the request that caused it
	defer func() { _ = db.Close() }()

	hasher := security.NewBcryptHasher(cfg.BcryptCost)
	issuer := security.NewJWTIssuer(cfg.JWTSecret, cfg.AccessTTL)
	userRepo := postgres.NewUserRepository(db)
	tokenRepo := postgres.NewRefreshTokenRepository(db)

	authSvc, err := application.NewAuthService(userRepo, tokenRepo, hasher, issuer, cfg.RefreshTTL)
	if err != nil {
		return err
	}
	authSvc.WithRefreshGrace(cfg.RefreshGrace)

	app := transport.NewApp(transport.Options{
		Logger:         log,
		AllowedOrigins: cfg.AllowedOrigins,
		TrustedProxies: cfg.TrustedProxies,
		Verifier:       auth.NewVerifier([]byte(cfg.JWTSecret)),
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(ctx context.Context) error { return db.PingContext(ctx) },
		},
	}, transport.Services{
		Auth:      authSvc,
		Users:     application.NewUserService(userRepo),
		Addresses: application.NewAddressService(postgres.NewAddressRepository(db)),
	})

	ln, err := net.Listen("tcp", ":"+cfg.HTTPPort)
	if err != nil {
		return err
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listener(ln) }()
	log.Info("identity service started", "port", cfg.HTTPPort, "env", cfg.Env)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
