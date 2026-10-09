package http

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/shared/errorx"
)

// MapDomainError translates domain errors for httpx.NewErrorHandler.
func MapDomainError(err error) (*errorx.AppError, bool) {
	switch {
	case errors.Is(err, domain.ErrCategoryNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "Category not found"), true
	case errors.Is(err, domain.ErrProductNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "Product not found"), true
	case errors.Is(err, domain.ErrVariantNotFound):
		return errorx.WithMessage(errorx.ErrNotFound, "Variant not found"), true
	case errors.Is(err, domain.ErrSlugTaken):
		return errorx.WithMessage(errorx.ErrConflict, "Slug is already in use"), true
	case errors.Is(err, domain.ErrSKUTaken):
		return errorx.WithMessage(errorx.ErrConflict, "SKU is already in use"), true
	case errors.Is(err, domain.ErrVariantExists):
		return errorx.WithMessage(errorx.ErrConflict, "A variant with this size and color already exists"), true
	case errors.Is(err, domain.ErrCategoryInUse):
		return errorx.WithMessage(errorx.ErrConflict, "Category still has products or subcategories"), true
	case errors.Is(err, domain.ErrNoActiveVariant):
		return errorx.WithMessage(errorx.ErrConflict, "An active product needs at least one active variant"), true
	case errors.Is(err, domain.ErrInvalidSKU):
		return errorx.WithMessage(errorx.ErrValidation, "SKU may only contain letters, digits, '-' and '_' (2-64 characters)"), true
	case errors.Is(err, domain.ErrInvalidStatus):
		return errorx.WithMessage(errorx.ErrValidation, "Invalid status"), true
	}
	return nil, false
}
