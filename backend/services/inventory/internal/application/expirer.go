package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/repository"
)

// expireBatch caps one sweep, so a long backlog cannot hold a transaction open for minutes.
const expireBatch = 200

// Expirer gives stock back when a checkout is abandoned. Without it, a customer who
// starts paying and disappears would keep those units away from everyone else forever.
type Expirer struct {
	reservations repository.ReservationRepository
	log          *slog.Logger
	interval     time.Duration
}

func NewExpirer(reservations repository.ReservationRepository, log *slog.Logger, interval time.Duration) *Expirer {
	return &Expirer{reservations: reservations, log: log, interval: interval}
}

// Run sweeps until ctx is cancelled. The first sweep happens immediately, so
// reservations that expired while the service was down are recovered on startup.
func (e *Expirer) Run(ctx context.Context) {
	e.sweep(ctx)

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.sweep(ctx)
		}
	}
}

// RunOnce releases every reservation that is already due and returns how many.
func (e *Expirer) RunOnce(ctx context.Context) (int, error) {
	return e.reservations.ExpireDue(ctx, time.Now(), expireBatch)
}

func (e *Expirer) sweep(ctx context.Context) {
	released, err := e.RunOnce(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		// A failed sweep is retried on the next tick; the reservations stay due.
		e.log.ErrorContext(ctx, "reservation expiry sweep failed", "error", err, "released", released)
		return
	}
	if released > 0 {
		e.log.InfoContext(ctx, "expired reservations released", "count", released)
	}
}
