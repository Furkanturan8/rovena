package postgres

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/uptrace/bun/driver/pgdriver"
)

const uniqueViolation = "23505"

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
	if code == uniqueViolation && constraint == "payments_one_success_per_order_idx" {
		return domain.ErrOrderAlreadyPaid
	}
	return err
}
