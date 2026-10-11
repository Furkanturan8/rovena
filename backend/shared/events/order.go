package events

import "github.com/google/uuid"

const (
	// OrderConfirmed and OrderCancelled are facts published by Order when the checkout
	// saga reaches its end. Nothing consumes them yet; they are the hook for
	// notifications and analytics.
	OrderConfirmed = "OrderConfirmed"
	OrderCancelled = "OrderCancelled"
)

const (
	OrderConfirmedVersion = 1
	OrderCancelledVersion = 1
)

type OrderConfirmedPayload struct {
	OrderID    uuid.UUID `json:"order_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	TotalCents int64     `json:"total_cents"`
	Currency   string    `json:"currency"`
}

type OrderCancelledPayload struct {
	OrderID    uuid.UUID `json:"order_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	// Code is a stable reason: out_of_stock, item_unavailable, checkout_timeout,
	// payment_timeout, reservation_lost, or the payment failure code (card_declined, ...).
	Code    string `json:"code"`
	Message string `json:"message"`
}
