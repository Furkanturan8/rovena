package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	"github.com/Furkanturan8/rovena/services/inventory/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/inventory/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/inventory/internal/repository/postgres"
	transport "github.com/Furkanturan8/rovena/services/inventory/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "inventory:", err)
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
	log := logging.New("inventory", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	reservationRepo := postgres.NewReservationRepository(db)
	reservations := application.NewReservationService(reservationRepo, cfg.ReservationTTL)
	stock := application.NewStockService(postgres.NewStockRepository(db))

	app := transport.NewApp(transport.Options{
		Logger:         log,
		AllowedOrigins: cfg.AllowedOrigins,
		TrustedProxies: cfg.TrustedProxies,
		Verifier:       auth.NewVerifier([]byte(cfg.JWTSecret)),
		InternalAPIKey: cfg.InternalAPIKey,
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(ctx context.Context) error { return db.PingContext(ctx) },
		},
	}, transport.Services{
		Stock:        stock,
		Reservations: reservations,
	})

	// Expired holds are released in the background: a checkout that is abandoned
	// after reserving must not keep stock away from other customers.
	var wg sync.WaitGroup
	expirer := application.NewExpirer(reservationRepo, log, cfg.ExpiryInterval)
	wg.Add(1)
	go func() {
		defer wg.Done()
		expirer.Run(ctx)
	}()

	ln, err := net.Listen("tcp", ":"+cfg.HTTPPort)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listener(ln) }()
	log.Info("inventory service started",
		"port", cfg.HTTPPort, "env", cfg.Env,
		"reservation_ttl", cfg.ReservationTTL.String(), "expiry_interval", cfg.ExpiryInterval.String())

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
	wg.Wait()
	return nil
}
