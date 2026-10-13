package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type CategoryRepository struct {
	db *bun.DB
}

func NewCategoryRepository(db *bun.DB) *CategoryRepository { return &CategoryRepository{db: db} }

var _ repository.CategoryRepository = (*CategoryRepository)(nil)

func (r *CategoryRepository) Create(ctx context.Context, c *domain.Category) error {
	_, err := r.db.NewInsert().Model(categoryRowFrom(c)).Exec(ctx)
	if err != nil {
		return mapMissingCategory(err)
	}
	return nil
}

func (r *CategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	return r.getOne(ctx, "c.id = ?", id)
}

func (r *CategoryRepository) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	return r.getOne(ctx, "c.slug = ?", slug)
}

func (r *CategoryRepository) getOne(ctx context.Context, where string, arg any) (*domain.Category, error) {
	row := new(categoryRow)
	err := r.db.NewSelect().Model(row).Where(where, arg).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *CategoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	var rows []categoryRow
	if err := r.db.NewSelect().Model(&rows).OrderExpr("c.name ASC, c.id ASC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]domain.Category, len(rows))
	for i := range rows {
		out[i] = *rows[i].toDomain()
	}
	return out, nil
}

func (r *CategoryRepository) Update(ctx context.Context, c *domain.Category) error {
	c.UpdatedAt = time.Now()
	res, err := r.db.NewUpdate().Model((*categoryRow)(nil)).
		Set("name = ?", c.Name).
		Set("description = ?", c.Description).
		Set("translations = ?::jsonb", string(objectOrEmpty(c.Translations))).
		Set("updated_at = ?", c.UpdatedAt).
		Where("id = ?", c.ID).
		Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.NewDelete().Model((*categoryRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		if code, _ := pgCode(err); code == foreignKeyViolation {
			return domain.ErrCategoryInUse
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

func (r *CategoryRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	return r.db.NewSelect().Model((*categoryRow)(nil)).Where("c.slug = ?", slug).Exists(ctx)
}
