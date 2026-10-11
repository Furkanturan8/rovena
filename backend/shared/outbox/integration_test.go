//go:build integration

package outbox_test

// Runs against a real PostgreSQL and RabbitMQ:
//   ROVENA_TEST_DATABASE_URL=postgres://postgres:...@localhost:5433/postgres?sslmode=disable \
//   ROVENA_TEST_AMQP_URL=amqp://user:pass@localhost:5672/ go test -tags integration ./outbox
// Each test works in its own PostgreSQL schema (dropped afterwards) and its own queues. The
// database user needs permission to create schemas.

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/outbox"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func newDB(t *testing.T) *bun.DB {
	t.Helper()
	dsn := os.Getenv("ROVENA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ROVENA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	schema := "outbox_test_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	admin := bun.NewDB(sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn))), pgdialect.New())
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	db := bun.NewDB(sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN(dsn),
		pgdriver.WithConnParams(map[string]any{"search_path": schema}),
	)), pgdialect.New())
	t.Cleanup(func() {
		db.Close()
		admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	if _, err := db.ExecContext(ctx, outbox.UpSQL()); err != nil {
		t.Fatal(err)
	}
	return db
}

type fakePublisher struct {
	mu     sync.Mutex
	sent   []messaging.Envelope
	failOn func(messaging.Envelope) error
}

func (f *fakePublisher) Publish(_ context.Context, env messaging.Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn != nil {
		if err := f.failOn(env); err != nil {
			return err
		}
	}
	f.sent = append(f.sent, env)
	return nil
}

func (f *fakePublisher) types() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	for i, e := range f.sent {
		out[i] = e.Type
	}
	return out
}

func emit(t *testing.T, db bun.IDB, typ string) messaging.Envelope {
	t.Helper()
	env, err := outbox.Emit(context.Background(), db, typ, 1, map[string]string{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func count(t *testing.T, db *bun.DB, where string) int {
	t.Helper()
	var n int
	if err := db.NewRaw("SELECT count(*) FROM outbox_events WHERE "+where).Scan(context.Background(), &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEventAndBusinessChangeCommitOrRollBackTogether(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	if _, err := db.ExecContext(ctx, "CREATE TABLE orders (id uuid PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}

	rollback := errors.New("business rule failed after the event was added")
	err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO orders VALUES (?)", uuid.New()); err != nil {
			return err
		}
		emit(t, tx, "OrderCreated")
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("unexpected error: %v", err)
	}
	var orders int
	db.NewRaw("SELECT count(*) FROM orders").Scan(ctx, &orders)
	if orders != 0 || count(t, db, "true") != 0 {
		t.Fatalf("rollback left %d orders and %d events behind", orders, count(t, db, "true"))
	}

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO orders VALUES (?)", uuid.New()); err != nil {
			return err
		}
		emit(t, tx, "OrderCreated")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	db.NewRaw("SELECT count(*) FROM orders").Scan(ctx, &orders)
	if orders != 1 || count(t, db, "published_at IS NULL") != 1 {
		t.Fatal("commit must keep both the order and its pending event")
	}
}

func TestRelayPublishesOnceAndMarksRows(t *testing.T) {
	db := newDB(t)
	pub := &fakePublisher{}
	relay := outbox.NewRelay(db, pub, nil)
	emit(t, db, "A")
	emit(t, db, "B")

	n, err := relay.RunOnce(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("first run: %d, %v", n, err)
	}
	n, err = relay.RunOnce(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("second run must find nothing: %d, %v", n, err)
	}
	if got := pub.types(); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("sent %v", got)
	}
	if count(t, db, "published_at IS NOT NULL") != 2 {
		t.Fatal("rows were not marked as published")
	}
}

func TestRelayKeepsOrderAndRetriesAfterBrokerFailure(t *testing.T) {
	db := newDB(t)
	broken := true
	pub := &fakePublisher{failOn: func(e messaging.Envelope) error {
		if broken && e.Type == "B" {
			return errors.New("broker unavailable")
		}
		return nil
	}}
	relay := outbox.NewRelay(db, pub, nil)
	emit(t, db, "A")
	emit(t, db, "B")
	emit(t, db, "C")

	n, err := relay.RunOnce(context.Background())
	if err == nil || n != 1 {
		t.Fatalf("expected one event sent and an error, got %d, %v", n, err)
	}
	// C must not overtake B.
	if got := pub.types(); len(got) != 1 || got[0] != "A" {
		t.Fatalf("sent %v", got)
	}
	var attempts int
	var lastErr string
	db.NewRaw("SELECT attempts, last_error FROM outbox_events WHERE type = 'B'").Scan(context.Background(), &attempts, &lastErr)
	if attempts != 1 || lastErr != "broker unavailable" {
		t.Fatalf("failure not recorded: attempts=%d error=%q", attempts, lastErr)
	}

	broken = false
	n, err = relay.RunOnce(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("recovery run: %d, %v", n, err)
	}
	if got := pub.types(); len(got) != 3 || got[1] != "B" || got[2] != "C" {
		t.Fatalf("order after recovery: %v", got)
	}
}

func TestRelayHandlesBacklogLargerThanOneBatch(t *testing.T) {
	db := newDB(t)
	pub := &fakePublisher{}
	for i := 0; i < 120; i++ {
		emit(t, db, "E")
	}
	n, err := outbox.NewRelay(db, pub, nil).RunOnce(context.Background())
	if err != nil || n != 120 {
		t.Fatalf("published %d, %v", n, err)
	}
}

func TestConcurrentRelaysNeverSendTheSameRowTwice(t *testing.T) {
	db := newDB(t)
	pub := &fakePublisher{}
	for i := 0; i < 100; i++ {
		emit(t, db, "E")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := outbox.NewRelay(db, pub, nil).RunOnce(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	seen := map[uuid.UUID]bool{}
	for _, e := range pub.sent {
		if seen[e.ID] {
			t.Fatalf("event %s sent twice", e.ID)
		}
		seen[e.ID] = true
	}
	if len(seen) != 100 {
		t.Fatalf("only %d of 100 events were sent", len(seen))
	}
}

func TestInboxRunsEachEventOnce(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	inbox := outbox.NewInbox(db, "payments")
	env, _ := messaging.NewEnvelope(ctx, "T", 1, struct{}{})

	var runs int
	fn := func(context.Context, bun.Tx) error { runs++; return nil }
	for i := 0; i < 3; i++ {
		if _, err := inbox.Handle(ctx, env, fn); err != nil {
			t.Fatal(err)
		}
	}
	if runs != 1 {
		t.Fatalf("handler ran %d times for one event", runs)
	}

	// The same event for a different consumer is a different job.
	if ran, err := outbox.NewInbox(db, "emails").Handle(ctx, env, fn); err != nil || !ran {
		t.Fatalf("another consumer must still process it: %v, %v", ran, err)
	}
}

func TestInboxForgetsEventWhenHandlerFails(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	inbox := outbox.NewInbox(db, "payments")
	env, _ := messaging.NewEnvelope(ctx, "T", 1, struct{}{})

	if _, err := inbox.Handle(ctx, env, func(context.Context, bun.Tx) error { return errors.New("boom") }); err == nil {
		t.Fatal("expected the handler error")
	}
	ran, err := inbox.Handle(ctx, env, func(context.Context, bun.Tx) error { return nil })
	if err != nil || !ran {
		t.Fatalf("a failed attempt must not count as handled: ran=%v err=%v", ran, err)
	}
}

func TestInboxIsSafeUnderConcurrentDuplicateDelivery(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	inbox := outbox.NewInbox(db, "payments")
	env, _ := messaging.NewEnvelope(ctx, "T", 1, struct{}{})

	var runs atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := inbox.Handle(ctx, env, func(context.Context, bun.Tx) error {
				runs.Add(1)
				time.Sleep(50 * time.Millisecond) // widen the race window
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if runs.Load() != 1 {
		t.Fatalf("handler ran %d times", runs.Load())
	}
}

// lostConfirmPublisher models a relay crash at the worst moment: the broker has the
// event, but the relay never learns that and does not mark the row.
type lostConfirmPublisher struct {
	inner outbox.Publisher
	lost  atomic.Bool
}

func (p *lostConfirmPublisher) Publish(ctx context.Context, env messaging.Envelope) error {
	if err := p.inner.Publish(ctx, env); err != nil {
		return err
	}
	if p.lost.CompareAndSwap(true, false) {
		return errors.New("connection lost before the confirm arrived")
	}
	return nil
}

func TestRelayCrashAfterPublishDeliversTwiceButHandlesOnce(t *testing.T) {
	amqpURL := os.Getenv("ROVENA_TEST_AMQP_URL")
	if amqpURL == "" {
		t.Skip("ROVENA_TEST_AMQP_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := newDB(t)

	broker, err := messaging.Dial(ctx, amqpURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.Close()

	id := uuid.NewString()[:8]
	typ := "test.outbox." + id
	sub := messaging.Subscription{Queue: "test.outbox.q." + id, Types: []string{typ}, MaxRetries: 1, RetryDelay: 200 * time.Millisecond}
	defer func() {
		if conn, err := amqp.Dial(amqpURL); err == nil {
			if ch, err := conn.Channel(); err == nil {
				for _, q := range []string{sub.Queue, sub.Queue + ".retry", sub.Queue + ".dlq"} {
					ch.QueueDelete(q, false, false, false)
				}
			}
			conn.Close()
		}
	}()

	if _, err := db.ExecContext(ctx, "CREATE TABLE effects (event_id uuid)"); err != nil {
		t.Fatal(err)
	}
	var deliveries atomic.Int32
	inbox := outbox.NewInbox(db, "effects")
	idempotent := inbox.Handler(func(ctx context.Context, tx bun.Tx, env messaging.Envelope) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO effects VALUES (?)", env.ID)
		return err
	})
	// Counted outside the inbox, so duplicates that the inbox skips are still seen.
	handler := func(ctx context.Context, env messaging.Envelope) error {
		deliveries.Add(1)
		return idempotent(ctx, env)
	}
	cctx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); broker.Consume(cctx, sub, handler) }()
	defer func() { stop(); <-done }()
	time.Sleep(500 * time.Millisecond) // let the queue be declared before publishing

	if _, err := outbox.Emit(ctx, db, typ, 1, struct{}{}); err != nil {
		t.Fatal(err)
	}

	pub := &lostConfirmPublisher{inner: broker}
	pub.lost.Store(true)
	relay := outbox.NewRelay(db, pub, nil)
	if _, err := relay.RunOnce(ctx); err == nil {
		t.Fatal("the first run should report the lost confirm")
	}
	if count(t, db, "published_at IS NULL") != 1 {
		t.Fatal("the row must stay pending so the relay tries again")
	}
	if n, err := relay.RunOnce(ctx); err != nil || n != 1 {
		t.Fatalf("second run: %d, %v", n, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for deliveries.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if deliveries.Load() != 2 {
		t.Fatalf("expected the broker to deliver the event twice, got %d", deliveries.Load())
	}
	var effects int
	db.NewRaw("SELECT count(*) FROM effects").Scan(ctx, &effects)
	if effects != 1 {
		t.Fatalf("the duplicate delivery caused %d effects", effects)
	}
}
