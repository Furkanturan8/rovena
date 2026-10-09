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

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/catalog/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository/postgres"
	transport "github.com/Furkanturan8/rovena/services/catalog/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "catalog:", err)
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
	log := logging.New("catalog", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	app := transport.NewApp(transport.Options{
		Logger:         log,
		AllowedOrigins: cfg.AllowedOrigins,
		TrustedProxies: cfg.TrustedProxies,
		Verifier:       auth.NewVerifier([]byte(cfg.JWTSecret)),
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(ctx context.Context) error { return db.PingContext(ctx) },
		},
	}, transport.Services{
		Categories: application.NewCategoryService(postgres.NewCategoryRepository(db)),
		Products:   application.NewProductService(postgres.NewProductRepository(db), postgres.NewVariantRepository(db)),
	})

	ln, err := net.Listen("tcp", ":"+cfg.HTTPPort)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listener(ln) }()
	log.Info("catalog service started", "port", cfg.HTTPPort, "env", cfg.Env)

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
