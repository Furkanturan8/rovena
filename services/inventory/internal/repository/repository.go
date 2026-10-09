package repository

import (
	"context"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/google/uuid"
)

type StockRepository interface {
	// Create stores the stock item and, when OnHand is positive, the opening
	// receipt movement in the same transaction.
	// Returns ErrStockItemExists or ErrSKUTaken on conflict.
	Create(ctx context.Context, item *domain.StockItem, reason string) error
	GetByVariantID(ctx context.Context, variantID uuid.UUID) (*domain.StockItem, error)
	List(ctx context.Context, f domain.StockFilter) ([]domain.StockItem, int, error)
	// Availability returns one entry per known variant; unknown variants are left out.
	Availability(ctx context.Context, variantIDs []uuid.UUID) ([]domain.Availability, error)
	// ApplyDelta changes on_hand by delta and records a movement, in one transaction.
	// The update is conditional, so a concurrent reservation can never be invalidated:
	// it returns ErrReservedExceedsOnHand instead of letting on_hand fall below reserved.
	ApplyDelta(ctx context.Context, variantID uuid.UUID, delta int, typ domain.MovementType, reason string) (*domain.StockItem, error)
	ListMovements(ctx context.Context, f domain.MovementFilter) ([]domain.Movement, int, error)
}

type ReservationRepository interface {
	// Reserve stores the reservation and raises the reserved quantities in one
	// transaction, using conditional updates so two concurrent orders can never
	// reserve the same unit. Returns a *domain.StockError when an item cannot be held.
	//
	// Reserving is idempotent per order: when the order already has a reservation,
	// that one is returned with created=false and no stock is held twice.
	Reserve(ctx context.Context, r *domain.Reservation) (res *domain.Reservation, created bool, err error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Reservation, error)
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Reservation, error)
	// Commit turns the held stock into a sale: on_hand and reserved both drop and a
	// sale movement is written per item. Committing twice is a no-op.
	// Returns ErrReservationClosed for a released or expired reservation.
	Commit(ctx context.Context, id uuid.UUID) (*domain.Reservation, error)
	// Release gives the held stock back. Releasing twice is a no-op; a committed
	// reservation cannot be released (ErrReservationCommitted).
	Release(ctx context.Context, id uuid.UUID) (*domain.Reservation, error)
	// ExpireDue releases up to limit reservations whose expiry has passed and
	// returns how many were released.
	ExpireDue(ctx context.Context, now time.Time, limit int) (int, error)
}
