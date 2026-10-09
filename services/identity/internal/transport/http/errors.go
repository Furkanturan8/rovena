package http

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/shared/errorx"
)

// MapDomainError translates domain errors for httpx.NewErrorHandler.
func MapDomainError(err error) (*errorx.AppError, bool) {
	switch {
	case errors.Is(err, domain.ErrEmailTaken):
		return errorx.WithMessage(errorx.ErrConflict, "Email is already registered"), true
	case errors.Is(err, domain.ErrUserNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "User not found"), true
	case errors.Is(err, domain.ErrInvalidCredentials):
		return errorx.WithMessage(errorx.ErrInvalidCredentials, "Invalid email or password"), true
	case errors.Is(err, domain.ErrInvalidRefreshToken):
		return errorx.WithMessage(errorx.ErrUnauthorized, "Invalid or expired refresh token"), true
	case errors.Is(err, domain.ErrInvalidRole):
		return errorx.WithMessage(errorx.ErrValidation, "Invalid role"), true
	case errors.Is(err, domain.ErrLastAdmin):
		return errorx.WithMessage(errorx.ErrConflict, "Cannot demote the last admin"), true
	case errors.Is(err, domain.ErrPasswordTooLong):
		return errorx.WithMessage(errorx.ErrValidation, "Password must be at most 72 bytes"), true
	}
	return nil, false
}
