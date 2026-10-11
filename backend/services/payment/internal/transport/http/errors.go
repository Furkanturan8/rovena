package http

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/shared/errorx"
)

// MapDomainError translates domain errors for httpx.NewErrorHandler.
func MapDomainError(err error) (*errorx.AppError, bool) {
	switch {
	case errors.Is(err, domain.ErrPaymentNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "Payment not found"), true
	}
	return nil, false
}
