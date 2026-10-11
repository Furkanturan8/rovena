package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/uptrace/bun"
)

const (
	defaultInterval = time.Second
	defaultBatch    = 50
)

// Publisher is what the relay needs from the broker; *messaging.Broker satisfies it.
type Publisher interface {
	Publish(ctx context.Context, env messaging.Envelope) error
}

// Relay delivers outbox rows to the broker. Delivery is at-least-once: if the process
// dies after the broker confirmed an event but before the row is marked as published,
// the event is sent again, which is why consumers deduplicate on the event ID.
//
// Run one relay per service. Several are safe from duplicates of the same batch
// (rows are locked with SKIP LOCKED) but may reorder events relative to each other.
type Relay struct {
	db       *bun.DB
	pub      Publisher
	log      *slog.Logger
	interval time.Duration
	batch    int
}

func NewRelay(db *bun.DB, pub Publisher, log *slog.Logger) *Relay {
	if log == nil {
		log = slog.Default()
	}
	return &Relay{db: db, pub: pub, log: log, interval: defaultInterval, batch: defaultBatch}
}

// WithInterval sets how often the outbox is polled.
func (r *Relay) WithInterval(d time.Duration) *Relay {
	r.interval = d
	return r
}

// Run publishes pending events until ctx is cancelled. It sweeps immediately, so events
// left behind by a crash are sent on startup.
func (r *Relay) Run(ctx context.Context) {
	r.sweep(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

// RunOnce publishes everything that is pending right now and returns how many events
// were sent. It stops at the first event the broker does not accept, so order is kept.
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	total := 0
	for {
		n, more, err := r.publishBatch(ctx)
		total += n
		if err != nil || !more {
			return total, err
		}
	}
}

func (r *Relay) sweep(ctx context.Context) {
	n, err := r.RunOnce(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		// The rows stay pending and are retried on the next tick.
		r.log.ErrorContext(ctx, "outbox publish failed", "error", err, "published", n)
		return
	}
	if n > 0 {
		r.log.DebugContext(ctx, "outbox events published", "count", n)
	}
}

// publishBatch sends up to one batch. more reports whether a full batch was found, so
// the caller should look again.
func (r *Relay) publishBatch(ctx context.Context) (sent int, more bool, err error) {
	var publishErr error
	txErr := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var rows []row
		if err := tx.NewSelect().Model(&rows).
			Where("published_at IS NULL").
			OrderExpr("seq").
			Limit(int64(r.batch)).
			For("UPDATE SKIP LOCKED").
			Scan(ctx); err != nil {
			return err
		}
		more = len(rows) == r.batch

		for _, rw := range rows {
			var env messaging.Envelope
			if err := json.Unmarshal([]byte(rw.Envelope), &env); err != nil {
				return fmt.Errorf("outbox row %d holds an unreadable envelope: %w", rw.Seq, err)
			}
			if err := r.pub.Publish(ctx, env); err != nil {
				publishErr = err
				// Recorded in the same transaction, so it is committed with the
				// rows that did go out before this one.
				_, err := tx.NewUpdate().Model((*row)(nil)).
					Set("attempts = attempts + 1").
					Set("last_error = ?", publishErr.Error()).
					Where("seq = ?", rw.Seq).
					Exec(ctx)
				return err
			}
			if _, err := tx.NewUpdate().Model((*row)(nil)).
				Set("published_at = now()").
				Where("seq = ?", rw.Seq).
				Exec(ctx); err != nil {
				return err
			}
			sent++
		}
		return nil
	})
	if txErr != nil {
		return sent, false, txErr
	}
	if publishErr != nil {
		return sent, false, publishErr
	}
	return sent, more, nil
}
