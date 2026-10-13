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
)

type AddressRepository struct {
	db *bun.DB
}

func NewAddressRepository(db *bun.DB) *AddressRepository { return &AddressRepository{db: db} }

var _ repository.AddressRepository = (*AddressRepository)(nil)

func (r *AddressRepository) List(ctx context.Context, userID uuid.UUID) ([]domain.Address, error) {
	var rows []addressRow
	err := r.db.NewSelect().Model(&rows).Where("a.user_id = ?", userID).OrderExpr("a.is_default DESC, a.created_at, a.id").Scan(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Address, len(rows))
	for i := range rows {
		out[i] = *rows[i].toDomain()
	}
	return out, nil
}

func (r *AddressRepository) Get(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error) {
	row := new(addressRow)
	err := r.db.NewSelect().Model(row).Where("a.id = ? AND a.user_id = ?", id, userID).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAddressNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

// lockUser serializes address changes of one user, so the "one default" and "at most N" rules
// hold even when two requests race. The lock is released when the transaction ends.
func lockUser(ctx context.Context, tx bun.Tx, userID uuid.UUID) error {
	var locked uuid.UUID
	return tx.NewSelect().Model((*userRow)(nil)).Column("id").Where("id = ?", userID).For("UPDATE").Scan(ctx, &locked)
}

func (r *AddressRepository) Create(ctx context.Context, a *domain.Address) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUser(ctx, tx, a.UserID); err != nil {
			return err
		}
		count, err := tx.NewSelect().Model((*addressRow)(nil)).Where("a.user_id = ?", a.UserID).Count(ctx)
		if err != nil {
			return err
		}
		if count >= domain.MaxAddressesPerUser {
			return domain.ErrTooManyAddresses
		}
		if count == 0 {
			a.IsDefault = true
		}
		if a.IsDefault {
			if err := clearDefault(ctx, tx, a.UserID); err != nil {
				return err
			}
		}
		_, err = tx.NewInsert().Model(addressRowFrom(a)).Exec(ctx)
		return err
	})
}

func clearDefault(ctx context.Context, tx bun.Tx, userID uuid.UUID) error {
	_, err := tx.NewUpdate().Model((*addressRow)(nil)).Set("is_default = false").Where("user_id = ? AND is_default", userID).Exec(ctx)
	return err
}

func (r *AddressRepository) Update(ctx context.Context, a *domain.Address, makeDefault bool) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUser(ctx, tx, a.UserID); err != nil {
			return err
		}
		if makeDefault {
			if err := clearDefault(ctx, tx, a.UserID); err != nil {
				return err
			}
			a.IsDefault = true
		}
		a.UpdatedAt = time.Now()
		res, err := tx.NewUpdate().Model((*addressRow)(nil)).
			Set("title = ?", a.Title).Set("full_name = ?", a.FullName).Set("phone = ?", a.Phone).
			Set("line1 = ?", a.Line1).Set("line2 = ?", a.Line2).Set("district = ?", a.District).
			Set("city = ?", a.City).Set("postal_code = ?", a.PostalCode).
			Set("is_default = ?", a.IsDefault).Set("updated_at = ?", a.UpdatedAt).
			Where("id = ? AND user_id = ?", a.ID, a.UserID).
			Exec(ctx)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return domain.ErrAddressNotFound
		}
		return nil
	})
}

func (r *AddressRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		var wasDefault bool
		err := tx.NewSelect().Model((*addressRow)(nil)).Column("is_default").Where("id = ? AND user_id = ?", id, userID).Scan(ctx, &wasDefault)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrAddressNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*addressRow)(nil)).Where("id = ? AND user_id = ?", id, userID).Exec(ctx); err != nil {
			return err
		}
		if !wasDefault {
			return nil
		}
		// The newest remaining address takes over as default (no-op when none is left).
		_, err = tx.ExecContext(ctx, `UPDATE addresses SET is_default = true
			WHERE id = (SELECT id FROM addresses WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT 1)`, userID)
		return err
	})
}

func (r *AddressRepository) SetDefault(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error) {
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		exists, err := tx.NewSelect().Model((*addressRow)(nil)).Where("id = ? AND user_id = ?", id, userID).Exists(ctx)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrAddressNotFound
		}
		if err := clearDefault(ctx, tx, userID); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model((*addressRow)(nil)).Set("is_default = true").Set("updated_at = ?", time.Now()).Where("id = ?", id).Exec(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, userID, id)
}
