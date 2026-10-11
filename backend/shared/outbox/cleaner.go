package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/shared/telemetry"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	defaultCleanInterval = time.Hour
	cleanBatch           = 1000
)

var cleaned = telemetry.Counter("outbox.cleaned.rows", "Rows deleted from the outbox and inbox tables, by table")

// Cleaner deletes what the outbox and inbox no longer need, so the tables do not grow forever:
//
//   - published outbox rows older than the retention (a published event is delivered; the row is
//     only kept for a while to look at when something is investigated);
//   - inbox markers older than the retention.
//
// Unpublished outbox rows are never touched, however old: they are events nobody has heard yet.
//
// The retention must be longer than the longest time a message can be redelivered. The inbox marker
// is what recognises a redelivery, so removing it earlier would let an old duplicate run again.
// With retries measured in seconds and a dead-letter queue that is replayed by hand, days is safe.
type Cleaner struct {
	db        *bun.DB
	log       *slog.Logger
	retention time.Duration
	interval  time.Duration
	now       func() time.Time
}

func NewCleaner(db *bun.DB, log *slog.Logger, retention time.Duration) *Cleaner {
	if log == nil {
		log = slog.Default()
	}
	return &Cleaner{db: db, log: log, retention: retention, interval: defaultCleanInterval, now: time.Now}
}

// WithInterval sets how often the tables are cleaned.
func (c *Cleaner) WithInterval(d time.Duration) *Cleaner {
	c.interval = d
	return c
}

// Run cleans immediately and then on every tick, until ctx is cancelled.
func (c *Cleaner) Run(ctx context.Context) {
	c.sweep(ctx)
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.sweep(ctx)
		}
	}
}

func (c *Cleaner) sweep(ctx context.Context) {
	out, in, err := c.RunOnce(ctx)
	if err != nil {
		if ctx.Err() == nil {
			c.log.ErrorContext(ctx, "outbox cleanup failed", "error", err)
		}
		return
	}
	if out+in > 0 {
		c.log.InfoContext(ctx, "outbox cleanup", "outbox_rows", out, "inbox_rows", in, "retention", c.retention.String())
	}
}

// RunOnce deletes everything older than the retention and returns how many rows went from each
// table. It works in batches so a large backlog does not hold a long lock.
func (c *Cleaner) RunOnce(ctx context.Context) (outboxRows, inboxRows int64, err error) {
	cutoff := c.now().Add(-c.retention)
	outboxRows, err = c.deleteInBatches(ctx,
		`DELETE FROM outbox_events WHERE seq IN (
			SELECT seq FROM outbox_events WHERE published_at IS NOT NULL AND published_at < ? ORDER BY seq LIMIT ?)`,
		"outbox_events", cutoff)
	if err != nil {
		return outboxRows, 0, err
	}
	inboxRows, err = c.deleteInBatches(ctx,
		`DELETE FROM processed_events WHERE ctid IN (
			SELECT ctid FROM processed_events WHERE processed_at < ? LIMIT ?)`,
		"processed_events", cutoff)
	return outboxRows, inboxRows, err
}

func (c *Cleaner) deleteInBatches(ctx context.Context, query, table string, cutoff time.Time) (int64, error) {
	var total int64
	for {
		res, err := c.db.ExecContext(ctx, query, cutoff, cleanBatch)
		if err != nil {
			return total, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
		if n > 0 {
			cleaned.Add(ctx, n, metric.WithAttributes(attribute.String("table", table)))
		}
		if n < cleanBatch {
			return total, nil
		}
	}
}
