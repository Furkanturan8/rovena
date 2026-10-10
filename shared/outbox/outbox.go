// Package outbox implements the Transactional Outbox (reliable publishing) and its
// counterpart, the idempotent inbox (safe consuming), on top of PostgreSQL and bun.
//
// Tables are created by schema.sql, which each service copies into its own migrations.
package outbox

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//go:embed schema.sql
var schema string

// UpSQL returns the SQL of the "goose Up" section of schema.sql, for tests and tools
// that create the tables without goose.
func UpSQL() string {
	up, _, _ := strings.Cut(schema, "-- +goose Down")
	return strings.Replace(up, "-- +goose Up", "", 1)
}

type row struct {
	bun.BaseModel `bun:"table:outbox_events"`

	Seq         int64      `bun:",pk,autoincrement"`
	ID          uuid.UUID  `bun:"id,type:uuid"`
	Type        string     `bun:"type"`
	Envelope    string     `bun:"envelope,type:jsonb"`
	CreatedAt   time.Time  `bun:"created_at,nullzero,default:now()"`
	PublishedAt *time.Time `bun:"published_at"`
	Attempts    int        `bun:"attempts"`
	LastError   string     `bun:"last_error"`
}

// Add stores env in the outbox. Pass the transaction that performs the business change
// as db: that shared transaction is the whole point. Calling it with a plain *bun.DB
// works but gives up the guarantee.
func Add(ctx context.Context, db bun.IDB, env messaging.Envelope) error {
	if err := env.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	_, err = db.NewInsert().Model(&row{
		ID:       env.ID,
		Type:     env.Type,
		Envelope: string(body),
	}).Exec(ctx)
	return err
}

// Emit builds an event from payload and stores it in the outbox in one step.
func Emit(ctx context.Context, db bun.IDB, eventType string, version int, payload any) (messaging.Envelope, error) {
	env, err := messaging.NewEnvelope(ctx, eventType, version, payload)
	if err != nil {
		return messaging.Envelope{}, err
	}
	return env, Add(ctx, db, env)
}
