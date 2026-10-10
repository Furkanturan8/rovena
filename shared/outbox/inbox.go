package outbox

import (
	"context"

	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/uptrace/bun"
)

// Inbox makes event handling idempotent. The "event was handled" marker is written in the
// same transaction as the handler's own changes, so the two cannot disagree: either both
// are committed or neither, and a redelivered event finds the marker and does nothing.
type Inbox struct {
	db       *bun.DB
	consumer string
}

// NewInbox creates an inbox for one consumer. The name keeps markers apart when several
// consumers in one service handle the same event (give each queue its own name).
func NewInbox(db *bun.DB, consumer string) *Inbox {
	return &Inbox{db: db, consumer: consumer}
}

// Handle runs fn in a transaction unless env was already handled. It reports whether fn
// ran; a duplicate returns (false, nil). If fn fails, nothing is recorded, so the event
// is retried from scratch.
func (i *Inbox) Handle(ctx context.Context, env messaging.Envelope, fn func(ctx context.Context, tx bun.Tx) error) (bool, error) {
	ran := false
	err := i.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// If another delivery of the same event is mid-transaction, this insert waits for
		// it to finish, then either inserts (it rolled back) or finds the row (it committed).
		res, err := tx.NewRaw(
			"INSERT INTO processed_events (consumer, event_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
			i.consumer, env.ID,
		).Exec(ctx)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		ran = true
		return fn(ctx, tx)
	})
	if err != nil {
		return false, err
	}
	return ran, nil
}

// Handler adapts fn into a messaging.Handler that skips events it has already handled.
func (i *Inbox) Handler(fn func(ctx context.Context, tx bun.Tx, env messaging.Envelope) error) messaging.Handler {
	return func(ctx context.Context, env messaging.Envelope) error {
		_, err := i.Handle(ctx, env, func(ctx context.Context, tx bun.Tx) error {
			return fn(ctx, tx, env)
		})
		return err
	}
}
