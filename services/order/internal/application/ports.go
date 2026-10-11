package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// VariantInfo is Catalog's current answer about one variant.
type VariantInfo struct {
	VariantID   uuid.UUID
	ProductID   uuid.UUID
	ProductName string
	SKU         string
	Size        string
	Color       string
	PriceCents  int64
	Currency    string
	Sellable    bool
}

// Catalog is the Order service's view of the product catalog. Variants Catalog does
// not know are absent from the result.
type Catalog interface {
	Lookup(ctx context.Context, variantIDs []uuid.UUID) ([]VariantInfo, error)
}

type ReservationLine struct {
	VariantID uuid.UUID
	Quantity  int
}

type Reservation struct {
	ID        uuid.UUID
	ExpiresAt time.Time
}

// StockShortage says which variant blocked a reservation. NotTracked is true when
// Inventory has no stock record for the variant at all.
type StockShortage struct {
	VariantID  uuid.UUID
	Requested  int
	Available  int
	NotTracked bool
}

func (e *StockShortage) Error() string { return "stock shortage" }

// ErrReservationGone means Inventory no longer holds the reservation: it expired, was
// released, or never existed.
var ErrReservationGone = errors.New("reservation is gone")

// Inventory is the synchronous half of the saga. Reserve is idempotent per order.
// Any error other than *StockShortage and ErrReservationGone means "could not get an
// answer"; the caller leaves the order as it is and the recovery job tries again.
type Inventory interface {
	Reserve(ctx context.Context, orderID uuid.UUID, lines []ReservationLine) (*Reservation, error)
	Commit(ctx context.Context, reservationID uuid.UUID) error
	// Release also succeeds when the reservation is already gone.
	Release(ctx context.Context, reservationID uuid.UUID) error
}
