package application

import (
	"context"
	"fmt"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
)

const maxSlugAttempts = 20

// uniqueSlug derives a slug from name and appends -2, -3, ... until exists reports it free.
// The unique constraint in the database remains the final guard against races.
func uniqueSlug(ctx context.Context, name string, exists func(context.Context, string) (bool, error)) (string, error) {
	base := domain.Slugify(name)
	candidate := base
	for i := 2; i <= maxSlugAttempts+1; i++ {
		taken, err := exists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", domain.ErrSlugTaken
}
