package http

import (
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
)

// ---- requests ----

type CreateStockRequest struct {
	VariantID string `json:"variant_id" validate:"required,uuid"`
	SKU       string `json:"sku" validate:"required,min=2,max=64"`
	OnHand    int    `json:"on_hand" validate:"min=0,max=1000000"`
	Reason    string `json:"reason" validate:"max=200"`
}

type ReceiveStockRequest struct {
	Quantity int    `json:"quantity" validate:"required,gt=0,max=1000000"`
	Reason   string `json:"reason" validate:"max=200"`
}

// AdjustStockRequest changes on-hand stock in either direction. A reason is
// mandatory: every correction has to be explainable afterwards from the ledger.
type AdjustStockRequest struct {
	Delta  int    `json:"delta" validate:"required,min=-1000000,max=1000000"`
	Reason string `json:"reason" validate:"required,max=200"`
}

type ReservationItemRequest struct {
	VariantID string `json:"variant_id" validate:"required,uuid"`
	Quantity  int    `json:"quantity" validate:"required,gt=0,max=10000"`
}

type CreateReservationRequest struct {
	OrderID string                   `json:"order_id" validate:"required,uuid"`
	Items   []ReservationItemRequest `json:"items" validate:"required,min=1,max=100,dive"`
}

// ---- responses ----

type StockResponse struct {
	VariantID string    `json:"variant_id"`
	SKU       string    `json:"sku"`
	OnHand    int       `json:"on_hand"`
	Reserved  int       `json:"reserved"`
	Available int       `json:"available"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ToStockResponse(s *domain.StockItem) StockResponse {
	return StockResponse{
		VariantID: s.VariantID.String(), SKU: s.SKU, OnHand: s.OnHand, Reserved: s.Reserved,
		Available: s.Available(), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

// AvailabilityResponse is the public shape: it says what can be bought, not how
// the warehouse arrived at that number.
type AvailabilityResponse struct {
	VariantID string `json:"variant_id"`
	Available int    `json:"available"`
	InStock   bool   `json:"in_stock"`
}

func ToAvailabilityResponse(a domain.Availability) AvailabilityResponse {
	return AvailabilityResponse{
		VariantID: a.VariantID.String(), Available: a.Available, InStock: a.Available > 0,
	}
}

type MovementResponse struct {
	ID            string    `json:"id"`
	VariantID     string    `json:"variant_id"`
	Type          string    `json:"type"`
	QuantityDelta int       `json:"quantity_delta"`
	Reason        string    `json:"reason"`
	ReservationID *string   `json:"reservation_id"`
	CreatedAt     time.Time `json:"created_at"`
}

func ToMovementResponse(m domain.Movement) MovementResponse {
	r := MovementResponse{
		ID: m.ID.String(), VariantID: m.VariantID.String(), Type: string(m.Type),
		QuantityDelta: m.QuantityDelta, Reason: m.Reason, CreatedAt: m.CreatedAt,
	}
	if m.ReservationID != nil {
		s := m.ReservationID.String()
		r.ReservationID = &s
	}
	return r
}

type ReservationItemResponse struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type ReservationResponse struct {
	ID        string                    `json:"id"`
	OrderID   string                    `json:"order_id"`
	Status    string                    `json:"status"`
	Items     []ReservationItemResponse `json:"items"`
	ExpiresAt time.Time                 `json:"expires_at"`
	CreatedAt time.Time                 `json:"created_at"`
	UpdatedAt time.Time                 `json:"updated_at"`
}

func ToReservationResponse(r *domain.Reservation) ReservationResponse {
	items := make([]ReservationItemResponse, len(r.Items))
	for i, it := range r.Items {
		items[i] = ReservationItemResponse{VariantID: it.VariantID.String(), Quantity: it.Quantity}
	}
	return ReservationResponse{
		ID: r.ID.String(), OrderID: r.OrderID.String(), Status: string(r.Status), Items: items,
		ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
