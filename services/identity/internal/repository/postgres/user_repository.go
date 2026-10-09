package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const uniqueViolation = "23505"

type UserRepository struct {
	db *bun.DB
}

func NewUserRepository(db *bun.DB) *UserRepository { return &UserRepository{db: db} }

var _ repository.UserRepository = (*UserRepository)(nil)

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	_, err := r.db.NewInsert().Model(userRowFrom(u)).Exec(ctx)
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) && pgErr.Field('C') == uniqueViolation {
		return domain.ErrEmailTaken
	}
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return r.getOne(ctx, "id = ?", id)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.getOne(ctx, "email = ?", email)
}

func (r *UserRepository) getOne(ctx context.Context, where string, arg any) (*domain.User, error) {
	row := new(userRow)
	err := r.db.NewSelect().Model(row).Where(where, arg).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *UserRepository) List(ctx context.Context, p repository.ListUsersParams) ([]domain.User, int, error) {
	var rows []userRow
	total, err := r.db.NewSelect().Model(&rows).
		OrderExpr("created_at DESC, id DESC").
		Limit(int64(p.PageSize)).
		Offset(int64((p.Page - 1) * p.PageSize)).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	users := make([]domain.User, len(rows))
	for i := range rows {
		users[i] = *rows[i].toDomain()
	}
	return users, int(total), nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().Model((*userRow)(nil)).
		Set("first_name = ?", u.FirstName).
		Set("last_name = ?", u.LastName).
		Set("updated_at = ?", u.UpdatedAt).
		Where("id = ?", u.ID).
		Exec(ctx)
	return err
}

func (r *UserRepository) UpdateRole(ctx context.Context, id uuid.UUID, role domain.Role) error {
	res, err := r.db.NewUpdate().Model((*userRow)(nil)).
		Set("role = ?", string(role)).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) CountByRole(ctx context.Context, role domain.Role) (int, error) {
	n, err := r.db.NewSelect().Model((*userRow)(nil)).Where("role = ?", string(role)).Count(ctx)
	return int(n), err
}
