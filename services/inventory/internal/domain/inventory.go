package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaxReservationItems caps how many distinct variants a single reservation may hold.
const MaxReservationItems = 100

// StockItem is the stock of one catalog variant. OnHand is what is physically in
// the warehouse; Reserved is the part of it that is already promised to orders.
type StockItem struct {
	ID        uuid.UUID
	VariantID uuid.UUID
	SKU       string
	OnHand    int
	Reserved  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Available is what a new order may still take.
func (s StockItem) Available() int { return s.OnHand - s.Reserved }

// Availability is the lightweight shape the storefront asks for.
type Availability struct {
	VariantID uuid.UUID
	SKU       string
	Available int
}

type ReservationStatus string

const (
	// ReservationPending holds stock and expires unless it is committed first.
	ReservationPending ReservationStatus = "pending"
	// ReservationCommitted means the order was paid: the stock left the warehouse.
	ReservationCommitted ReservationStatus = "committed"
	// ReservationReleased means the order was cancelled and the stock is available again.
	ReservationReleased ReservationStatus = "released"
	// ReservationExpired is a release done by the expirer instead of by the order service.
	ReservationExpired ReservationStatus = "expired"
)

func (s ReservationStatus) Valid() bool {
	switch s {
	case ReservationPending, ReservationCommitted, ReservationReleased, ReservationExpired:
		return true
	}
	return false
}

type ReservationItem struct {
	VariantID uuid.UUID
	Quantity  int
}

type Reservation struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	Status    ReservationStatus
	Items     []ReservationItem
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type MovementType string

const (
	// MovementReceipt is stock arriving at the warehouse.
	MovementReceipt MovementType = "receipt"
	// MovementAdjustment is a manual correction, for example after a stock count.
	MovementAdjustment MovementType = "adjustment"
	// MovementSale is stock leaving the warehouse because a reservation was committed.
	MovementSale MovementType = "sale"
)

// Movement is one entry of the append-only stock ledger. Only changes to OnHand
// are recorded; reserving does not move stock in or out of the warehouse.
type Movement struct {
	ID            uuid.UUID
	VariantID     uuid.UUID
	Type          MovementType
	QuantityDelta int
	Reason        string
	ReservationID *uuid.UUID
	CreatedAt     time.Time
}

type StockSort string

const (
	SortNewest       StockSort = "newest"
	SortSKU          StockSort = "sku"
	SortAvailableAsc StockSort = "available_asc"
)

func (s StockSort) Valid() bool {
	switch s {
	case SortNewest, SortSKU, SortAvailableAsc:
		return true
	}
	return false
}

type StockFilter struct {
	SKU string // case-insensitive prefix
	// Below keeps only items whose available quantity is under the given value.
	Below    *int
	Sort     StockSort
	Page     int
	PageSize int
}

type MovementFilter struct {
	VariantID uuid.UUID
	Page      int
	PageSize  int
}

var (
	ErrStockItemNotFound     = errors.New("stock item not found")
	ErrStockItemExists       = errors.New("stock item already exists for this variant")
	ErrSKUTaken              = errors.New("sku already in use")
	ErrInsufficientStock     = errors.New("insufficient stock")
	ErrReservedExceedsOnHand = errors.New("on hand would drop below the reserved quantity")
	// ErrStockConflict reports that a database CHECK stopped an impossible stock
	// change. Reaching it means two operations interleaved in a way the locks were
	// supposed to prevent, so the caller should retry rather than get a 500.
	ErrStockConflict        = errors.New("stock changed concurrently")
	ErrReservationNotFound  = errors.New("reservation not found")
	ErrReservationCommitted = errors.New("reservation is already committed")
	ErrReservationClosed    = errors.New("reservation is no longer pending")
	ErrNoItems              = errors.New("reservation needs at least one item")
	ErrTooManyItems         = errors.New("too many reservation items")
	ErrDuplicateVariant     = errors.New("variant listed more than once")
	ErrInvalidQuantity      = errors.New("quantity must be positive")
	ErrInvalidSKU           = errors.New("invalid sku")
)

// StockError says which variant made a reservation fail, so the order service can
// tell the customer what is out of stock. Err is ErrInsufficientStock or
// ErrStockItemNotFound, so errors.Is still works on it.
type StockError struct {
	VariantID uuid.UUID
	Requested int
	Available int
	Err       error
}

func (e *StockError) Error() string {
	if errors.Is(e.Err, ErrStockItemNotFound) {
		return fmt.Sprintf("%v: variant %s", e.Err, e.VariantID)
	}
	return fmt.Sprintf("%v: variant %s, requested %d, available %d", e.Err, e.VariantID, e.Requested, e.Available)
}

func (e *StockError) Unwrap() error { return e.Err }
