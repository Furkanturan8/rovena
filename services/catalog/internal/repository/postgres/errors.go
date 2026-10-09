package postgres

import (
	"errors"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
)

func pgCode(err error) (code, constraint string) {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('C'), pgErr.Field('n')
	}
	return "", ""
}

// mapUnique converts unique-constraint violations into domain errors.
// Other errors pass through unchanged.
func mapUnique(err error) error {
	code, constraint := pgCode(err)
	if code != uniqueViolation {
		return err
	}
	switch constraint {
	case "categories_slug_key", "products_slug_key":
		return domain.ErrSlugTaken
	case "variants_sku_key":
		return domain.ErrSKUTaken
	case "variants_product_options_key":
		return domain.ErrVariantExists
	}
	return err
}

// mapMissingCategory converts a foreign-key violation on insert/update
// (referenced category or parent does not exist) into ErrCategoryNotFound.
func mapMissingCategory(err error) error {
	if code, _ := pgCode(err); code == foreignKeyViolation {
		return domain.ErrCategoryNotFound
	}
	return mapUnique(err)
}
