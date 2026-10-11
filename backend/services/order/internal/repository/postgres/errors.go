package postgres

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
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

func mapConstraint(err error) error {
	code, constraint := pgCode(err)
	if code == uniqueViolation && constraint == "orders_customer_idempotency_key" {
		return domain.ErrDuplicateCheckout
	}
	return err
}
