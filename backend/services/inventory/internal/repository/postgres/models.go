package postgres

import (
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type stockItemRow struct {
	bun.BaseModel `bun:"table:stock_items,alias:s"`

	ID        uuid.UUID `bun:",pk,type:uuid"`
	VariantID uuid.UUID `bun:",type:uuid,notnull"`
	SKU       string    `bun:"sku,notnull"`
	OnHand    int       `bun:",notnull"`
	Reserved  int       `bun:",notnull"`
	CreatedAt time.Time `bun:",notnull"`
	UpdatedAt time.Time `bun:",notnull"`
}

func stockItemRowFrom(s *domain.StockItem) *stockItemRow {
	return &stockItemRow{
		ID: s.ID, VariantID: s.VariantID, SKU: s.SKU, OnHand: s.OnHand, Reserved: s.Reserved,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func (r *stockItemRow) toDomain() *domain.StockItem {
	return &domain.StockItem{
		ID: r.ID, VariantID: r.VariantID, SKU: r.SKU, OnHand: r.OnHand, Reserved: r.Reserved,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// availabilityRow is scanned from the availability query; available is computed.
type availabilityRow struct {
	VariantID uuid.UUID `bun:"variant_id,type:uuid"`
	SKU       string    `bun:"sku"`
	Available int       `bun:"available"`
}

type reservationRow struct {
	bun.BaseModel `bun:"table:reservations,alias:r"`

	ID        uuid.UUID `bun:",pk,type:uuid"`
	OrderID   uuid.UUID `bun:",type:uuid,notnull"`
	Status    string    `bun:",notnull"`
	ExpiresAt time.Time `bun:",notnull"`
	CreatedAt time.Time `bun:",notnull"`
	UpdatedAt time.Time `bun:",notnull"`
}

func reservationRowFrom(r *domain.Reservation) *reservationRow {
	return &reservationRow{
		ID: r.ID, OrderID: r.OrderID, Status: string(r.Status), ExpiresAt: r.ExpiresAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (r *reservationRow) toDomain() *domain.Reservation {
	return &domain.Reservation{
		ID: r.ID, OrderID: r.OrderID, Status: domain.ReservationStatus(r.Status),
		ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type reservationItemRow struct {
	bun.BaseModel `bun:"table:reservation_items,alias:ri"`

	ReservationID uuid.UUID `bun:",pk,type:uuid"`
	VariantID     uuid.UUID `bun:",pk,type:uuid"`
	Quantity      int       `bun:",notnull"`
}

type movementRow struct {
	bun.BaseModel `bun:"table:stock_movements,alias:m"`

	ID            uuid.UUID  `bun:",pk,type:uuid"`
	VariantID     uuid.UUID  `bun:",type:uuid,notnull"`
	Type          string     `bun:",notnull"`
	QuantityDelta int        `bun:",notnull"`
	Reason        string     `bun:",notnull"`
	ReservationID *uuid.UUID `bun:",type:uuid,nullzero"`
	CreatedAt     time.Time  `bun:",notnull"`
}

func (r *movementRow) toDomain() *domain.Movement {
	return &domain.Movement{
		ID: r.ID, VariantID: r.VariantID, Type: domain.MovementType(r.Type),
		QuantityDelta: r.QuantityDelta, Reason: r.Reason, ReservationID: r.ReservationID,
		CreatedAt: r.CreatedAt,
	}
}
