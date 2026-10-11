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

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/services/order/internal/infrastructure/clients"
	"github.com/Furkanturan8/rovena/services/order/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/order/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/order/internal/repository/postgres"
	"github.com/Furkanturan8/rovena/services/order/internal/transport/consumer"
	transport "github.com/Furkanturan8/rovena/services/order/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/logging"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/outbox"
	"github.com/Furkanturan8/rovena/shared/telemetry"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "order:", err)
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
	log := logging.New("order", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := telemetry.Setup(ctx, "order", log)
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
	defer func() { _ = db.Close() }()

	broker, err := messaging.Dial(ctx, cfg.RabbitMQURL, log)
	if err != nil {
		return err
	}
	defer func() { _ = broker.Close() }()

	repo := postgres.NewOrderRepository(db)
	catalog := clients.NewCatalog(cfg.CatalogURL, cfg.InternalAPIKey, cfg.ClientTimeout)
	inventory := clients.NewInventory(cfg.InventoryURL, cfg.InternalAPIKey, cfg.ClientTimeout)
	orders := application.NewOrderService(repo, catalog, inventory, log, application.Config{
		StuckAfter:         cfg.StuckAfter,
		PendingMaxAge:      cfg.PendingMaxAge,
		PaymentTimeout:     cfg.PaymentTimeout,
		MaxPaymentRequests: cfg.MaxPaymentRequests,
	})
	results := consumer.NewPaymentResultHandler(orders)

	app := transport.NewApp(transport.Options{
		Logger:         log,
		AllowedOrigins: cfg.AllowedOrigins,
		TrustedProxies: cfg.TrustedProxies,
		Verifier:       auth.NewVerifier([]byte(cfg.JWTSecret)),
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(ctx context.Context) error { return db.PingContext(ctx) },
		},
	}, transport.Services{
		Carts:   application.NewCartService(repo, catalog),
		Orders:  orders,
		Queries: application.NewQueryService(repo),
	})

	var wg sync.WaitGroup
	// Sends PaymentRequested, OrderConfirmed and OrderCancelled from the outbox.
	wg.Add(1)
	go func() {
		defer wg.Done()
		outbox.NewRelay(db, broker, log).Run(ctx)
	}()
	// Receives PaymentSucceeded and PaymentFailed.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := broker.Consume(ctx, consumer.Subscription(cfg.RetryDelay), results.Handle); err != nil {
			log.Error("consumer stopped", "error", err)
			stop()
		}
	}()
	// Repairs orders that got stuck between two steps of the saga.
	wg.Add(1)
	go func() {
		defer wg.Done()
		orders.RunRecovery(ctx, cfg.RecoveryInterval)
	}()

	ln, err := net.Listen("tcp", ":"+cfg.HTTPPort)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listener(ln) }()
	log.Info("order service started", "port", cfg.HTTPPort, "env", cfg.Env,
		"catalog", cfg.CatalogURL, "inventory", cfg.InventoryURL)

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
