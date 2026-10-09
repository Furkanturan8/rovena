package http

import (
	"errors"
	"strconv"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/shared/errorx"
)

// MapDomainError translates domain errors for httpx.NewErrorHandler.
func MapDomainError(err error) (*errorx.AppError, bool) {
	// A failed reservation names the variant that blocked it, so the order service
	// can tell the customer which line is the problem.
	var stockErr *domain.StockError
	if errors.As(err, &stockErr) {
		fields := map[string]string{"variant_id": stockErr.VariantID.String()}
		if errors.Is(stockErr.Err, domain.ErrStockItemNotFound) {
			return errorx.WithFields(
				errorx.WithMessage(errorx.ErrNotFound, "Stock is not tracked for one of the variants"), fields), true
		}
		fields["requested"] = strconv.Itoa(stockErr.Requested)
		fields["available"] = strconv.Itoa(stockErr.Available)
		return errorx.WithFields(
			errorx.WithMessage(errorx.ErrConflict, "Insufficient stock"), fields), true
	}

	switch {
	case errors.Is(err, domain.ErrStockItemNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "Stock item not found"), true
	case errors.Is(err, domain.ErrReservationNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "Reservation not found"), true
	case errors.Is(err, domain.ErrStockItemExists):
		return errorx.WithMessage(errorx.ErrConflict, "Stock is already tracked for this variant"), true
	case errors.Is(err, domain.ErrSKUTaken):
		return errorx.WithMessage(errorx.ErrConflict, "SKU is already in use"), true
	case errors.Is(err, domain.ErrReservedExceedsOnHand):
		return errorx.WithMessage(errorx.ErrConflict, "On-hand stock cannot drop below the reserved quantity"), true
	case errors.Is(err, domain.ErrStockConflict):
		return errorx.WithMessage(errorx.ErrConflict, "Stock changed concurrently, please retry"), true
	case errors.Is(err, domain.ErrReservationCommitted):
		return errorx.WithMessage(errorx.ErrConflict, "Reservation is already committed"), true
	case errors.Is(err, domain.ErrReservationClosed):
		return errorx.WithMessage(errorx.ErrConflict, "Reservation is no longer pending"), true
	case errors.Is(err, domain.ErrInvalidSKU):
		return errorx.WithMessage(errorx.ErrValidation, "SKU may only contain letters, digits, '-' and '_' (2-64 characters)"), true
	case errors.Is(err, domain.ErrInvalidQuantity):
		return errorx.WithMessage(errorx.ErrValidation, "Quantity must be a non-zero positive amount"), true
	case errors.Is(err, domain.ErrNoItems):
		return errorx.WithMessage(errorx.ErrValidation, "A reservation needs at least one item"), true
	case errors.Is(err, domain.ErrTooManyItems):
		return errorx.WithMessage(errorx.ErrValidation, "Too many reservation items"), true
	case errors.Is(err, domain.ErrDuplicateVariant):
		return errorx.WithMessage(errorx.ErrValidation, "Each variant may appear only once"), true
	}
	return nil, false
}
