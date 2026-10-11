package telemetry

import (
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/extra/bunotel"
)

// InstrumentBun returns db with a span for every database query to the trace of the request or message
// that caused it. The span carries the SQL with its placeholders, never the values: those can
// be emails, tokens or card references. Use the returned handle; it shares the connection pool
// with the one passed in.
func InstrumentBun(db *bun.DB) *bun.DB {
	return db.WithQueryHook(bunotel.NewQueryHook(
		bunotel.WithDBName(db.Dialect().Name().String()),
		bunotel.WithFormattedQueries(false),
	))
}
