package http

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/shared/errorx"
)

var errUnavailable = errorx.New(503, "A required service is temporarily unavailable, please try again")

// MapDomainError translates domain errors for httpx.NewErrorHandler.
func MapDomainError(err error) (*errorx.AppError, bool) {
	// A cart line that can not be ordered is named, so the client can highlight it.
	var item *domain.ItemError
	if errors.As(err, &item) {
		fields := map[string]string{"variant_id": item.VariantID.String()}
		return errorx.WithFields(
			errorx.WithMessage(errorx.ErrConflict, "An item in the cart is no longer available"), fields), true
	}

	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "Order not found"), true
	case errors.Is(err, domain.ErrCartEmpty):
		return errorx.WithMessage(errorx.ErrValidation, "The cart is empty"), true
	case errors.Is(err, domain.ErrCartChanged):
		return errorx.WithMessage(errorx.ErrConflict, "The cart changed during checkout, please review it and try again"), true
	case errors.Is(err, domain.ErrCartFull):
		return errorx.WithMessage(errorx.ErrConflict, "The cart has too many different items"), true
	case errors.Is(err, domain.ErrInvalidQuantity):
		return errorx.WithMessage(errorx.ErrValidation, "Quantity must be between 1 and 99"), true
	case errors.Is(err, domain.ErrIdempotencyKeyReused):
		return errorx.WithMessage(errorx.ErrValidation, "This Idempotency-Key was already used with a different payment method"), true
	case errors.Is(err, domain.ErrMixedCurrencies):
		return errorx.WithMessage(errorx.ErrValidation, "Items in different currencies can not be ordered together"), true
	case errors.Is(err, domain.ErrInvalidRequest):
		return errorx.WithMessage(errorx.ErrInvalidRequest, err.Error()), true
	case errors.Is(err, domain.ErrDependencyUnavailable):
		return errUnavailable, true
	}
	return nil, false
}
