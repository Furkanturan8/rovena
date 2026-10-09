package postgres

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	uniqueViolation     = "23505"
	checkViolation      = "23514"
	foreignKeyViolation = "23503"
)

func pgCode(err error) (code, constraint string) {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('C'), pgErr.Field('n')
	}
	return "", ""
}

// mapConstraint converts constraint violations into domain errors.
// Other errors pass through unchanged.
func mapConstraint(err error) error {
	code, constraint := pgCode(err)
	switch code {
	case uniqueViolation:
		switch constraint {
		case "stock_items_variant_id_key":
			return domain.ErrStockItemExists
		case "stock_items_sku_key":
			return domain.ErrSKUTaken
		}
	case checkViolation:
		// Reached only if a conditional update or a row lock was bypassed; these
		// CHECKs are the database-level guarantee that stock is never oversold.
		switch constraint {
		case "stock_items_reserved_within_on_hand":
			return domain.ErrReservedExceedsOnHand
		case "stock_items_on_hand_check", "stock_items_reserved_check":
			return domain.ErrStockConflict
		}
	case foreignKeyViolation:
		// reservation_items and stock_movements both point at stock_items.variant_id.
		return domain.ErrStockItemNotFound
	}
	return err
}
