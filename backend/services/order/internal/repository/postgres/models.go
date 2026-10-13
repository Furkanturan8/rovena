package postgres

import (
	"encoding/json"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type cartItemRow struct {
	bun.BaseModel `bun:"table:cart_items"`

	CustomerID uuid.UUID `bun:",pk,type:uuid"`
	VariantID  uuid.UUID `bun:",pk,type:uuid"`
	Quantity   int       `bun:",notnull"`
	AddedAt    time.Time `bun:",notnull,default:current_timestamp"`
}

type orderRow struct {
	bun.BaseModel `bun:"table:orders,alias:o"`

	ID               uuid.UUID       `bun:",pk,type:uuid"`
	CustomerID       uuid.UUID       `bun:",type:uuid,notnull"`
	Status           string          `bun:",notnull"`
	TotalCents       int64           `bun:",notnull"`
	Currency         string          `bun:",notnull"`
	IdempotencyKey   string          `bun:",notnull"`
	PaymentMethod    string          `bun:",notnull"`
	ReservationID    *uuid.UUID      `bun:",type:uuid"`
	ReservationState string          `bun:",notnull"`
	PaymentRequests  int             `bun:",notnull"`
	RefundState      string          `bun:",notnull"`
	RefundRequests   int             `bun:",notnull"`
	FailureCode      string          `bun:",notnull"`
	FailureMessage   string          `bun:",notnull"`
	ShippingAddress  json.RawMessage `bun:"shipping_address,type:jsonb,notnull"`
	Carrier          string          `bun:",notnull"`
	TrackingCode     string          `bun:",notnull"`
	ShippedAt        *time.Time
	DeliveredAt      *time.Time
	CreatedAt        time.Time `bun:",notnull"`
	UpdatedAt        time.Time `bun:",notnull"`
}

func orderRowFrom(o *domain.Order) *orderRow {
	if o.RefundState == "" {
		o.RefundState = domain.RefundNone
	}
	addr, _ := json.Marshal(o.ShippingAddress)
	return &orderRow{
		ShippingAddress: addr, Carrier: o.Carrier, TrackingCode: o.TrackingCode,
		ShippedAt: o.ShippedAt, DeliveredAt: o.DeliveredAt,
		ID: o.ID, CustomerID: o.CustomerID, Status: string(o.Status), TotalCents: o.TotalCents,
		Currency: o.Currency, IdempotencyKey: o.IdempotencyKey, PaymentMethod: o.PaymentMethod,
		ReservationID: o.ReservationID, ReservationState: string(o.ReservationState),
		PaymentRequests: o.PaymentRequests, RefundState: string(o.RefundState), RefundRequests: o.RefundRequests, FailureCode: o.FailureCode, FailureMessage: o.FailureMessage,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

func (r *orderRow) toDomain() domain.Order {
	var addr domain.ShippingAddress
	_ = json.Unmarshal(r.ShippingAddress, &addr)
	return domain.Order{
		ShippingAddress: addr, Carrier: r.Carrier, TrackingCode: r.TrackingCode,
		ShippedAt: r.ShippedAt, DeliveredAt: r.DeliveredAt,
		ID: r.ID, CustomerID: r.CustomerID, Status: domain.Status(r.Status), TotalCents: r.TotalCents,
		Currency: r.Currency, IdempotencyKey: r.IdempotencyKey, PaymentMethod: r.PaymentMethod,
		ReservationID: r.ReservationID, ReservationState: domain.ReservationState(r.ReservationState),
		PaymentRequests: r.PaymentRequests, RefundState: domain.RefundState(r.RefundState), RefundRequests: r.RefundRequests, FailureCode: r.FailureCode, FailureMessage: r.FailureMessage,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type orderItemRow struct {
	bun.BaseModel `bun:"table:order_items"`

	OrderID        uuid.UUID `bun:",pk,type:uuid"`
	VariantID      uuid.UUID `bun:",pk,type:uuid"`
	ProductID      uuid.UUID `bun:",type:uuid,notnull"`
	SKU            string    `bun:"sku,notnull"`
	ProductName    string    `bun:",notnull"`
	Size           string    `bun:",notnull"`
	Color          string    `bun:",notnull"`
	UnitPriceCents int64     `bun:",notnull"`
	Quantity       int       `bun:",notnull"`
}

func (r *orderItemRow) toDomain() domain.OrderItem {
	return domain.OrderItem{
		VariantID: r.VariantID, ProductID: r.ProductID, SKU: r.SKU, ProductName: r.ProductName,
		Size: r.Size, Color: r.Color, UnitPriceCents: r.UnitPriceCents, Quantity: r.Quantity,
	}
}

type historyRow struct {
	bun.BaseModel `bun:"table:order_status_history"`

	ID         int64     `bun:",pk,autoincrement"`
	OrderID    uuid.UUID `bun:",type:uuid,notnull"`
	FromStatus *string
	ToStatus   string    `bun:",notnull"`
	Reason     string    `bun:",notnull"`
	CreatedAt  time.Time `bun:",notnull,default:current_timestamp"`
}
