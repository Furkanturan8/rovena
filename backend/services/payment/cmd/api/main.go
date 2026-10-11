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

	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/services/payment/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/payment/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/payment/internal/infrastructure/provider"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository/postgres"
	"github.com/Furkanturan8/rovena/services/payment/internal/transport/consumer"
	transport "github.com/Furkanturan8/rovena/services/payment/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/logging"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/outbox"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "payment:", err)
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
	log := logging.New("payment", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	broker, err := messaging.Dial(ctx, cfg.RabbitMQURL, log)
	if err != nil {
		return err
	}
	defer broker.Close()

	repo := postgres.NewPaymentRepository(db)
	payments := application.NewPaymentService(repo, provider.NewFake(cfg.ProviderLatency), log, application.Config{
		MaxAttempts:   cfg.MaxAttempts,
		ChargeTimeout: cfg.ChargeTimeout,
	})
	handler := consumer.NewPaymentRequestedHandler(payments)

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
		Payments: application.NewQueryService(repo),
	})

	var wg sync.WaitGroup
	// Sends the PaymentSucceeded/PaymentFailed events that Complete stored in the outbox.
	wg.Add(1)
	go func() {
		defer wg.Done()
		outbox.NewRelay(db, broker, log).Run(ctx)
	}()
	// Receives PaymentRequested.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := broker.Consume(ctx, consumer.Subscription(cfg.MaxAttempts, cfg.RetryDelay), handler.Handle); err != nil {
			log.Error("consumer stopped", "error", err)
			stop()
		}
	}()

	ln, err := net.Listen("tcp", ":"+cfg.HTTPPort)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listener(ln) }()
	log.Info("payment service started", "port", cfg.HTTPPort, "env", cfg.Env,
		"max_attempts", cfg.MaxAttempts, "charge_timeout", cfg.ChargeTimeout.String())

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
