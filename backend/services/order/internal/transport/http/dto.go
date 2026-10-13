package http

import (
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/services/order/internal/domain"
)

// ---- requests ----

type SetCartItemRequest struct {
	Quantity int `json:"quantity" validate:"required,min=1,max=99"`
}

type CheckoutRequest struct {
	// PaymentMethod is a provider token for the card, never a card number.
	PaymentMethod string `json:"payment_method" validate:"required,max=128"`
	// ShippingAddress is copied into the order; later address edits do not touch it.
	ShippingAddress ShippingAddressDTO `json:"shipping_address" validate:"required"`
}

type ShippingAddressDTO struct {
	FullName   string `json:"full_name" validate:"required,max=100"`
	Phone      string `json:"phone" validate:"required,min=7,max=24"`
	Line1      string `json:"line1" validate:"required,max=200"`
	Line2      string `json:"line2" validate:"max=200"`
	District   string `json:"district" validate:"required,max=80"`
	City       string `json:"city" validate:"required,max=80"`
	PostalCode string `json:"postal_code" validate:"max=16"`
	Country    string `json:"country" validate:"max=2"`
}

func (a ShippingAddressDTO) toDomain() domain.ShippingAddress {
	country := a.Country
	if country == "" {
		country = "TR"
	}
	return domain.ShippingAddress{
		FullName: a.FullName, Phone: a.Phone, Line1: a.Line1, Line2: a.Line2,
		District: a.District, City: a.City, PostalCode: a.PostalCode, Country: country,
	}
}

type ShipRequest struct {
	Carrier      string `json:"carrier" validate:"required,max=60"`
	TrackingCode string `json:"tracking_code" validate:"required,max=60"`
}

// ---- responses ----

type CartLineResponse struct {
	VariantID      string `json:"variant_id"`
	ProductName    string `json:"product_name"`
	SKU            string `json:"sku"`
	Size           string `json:"size"`
	Color          string `json:"color"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Quantity       int    `json:"quantity"`
	LineTotalCents int64  `json:"line_total_cents"`
	Available      bool   `json:"available"`
}

type CartResponse struct {
	Items      []CartLineResponse `json:"items"`
	TotalCents int64              `json:"total_cents"`
	Currency   string             `json:"currency"`
}

func ToCartResponse(v *application.CartView) CartResponse {
	items := make([]CartLineResponse, len(v.Lines))
	for i, l := range v.Lines {
		items[i] = CartLineResponse{
			VariantID: l.VariantID.String(), ProductName: l.ProductName, SKU: l.SKU, Size: l.Size, Color: l.Color,
			UnitPriceCents: l.UnitPriceCents, Quantity: l.Quantity, LineTotalCents: l.LineTotalCents, Available: l.Available,
		}
	}
	return CartResponse{Items: items, TotalCents: v.TotalCents, Currency: v.Currency}
}

type OrderItemResponse struct {
	VariantID      string `json:"variant_id"`
	ProductID      string `json:"product_id"`
	SKU            string `json:"sku"`
	ProductName    string `json:"product_name"`
	Size           string `json:"size"`
	Color          string `json:"color"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Quantity       int    `json:"quantity"`
	LineTotalCents int64  `json:"line_total_cents"`
}

type OrderResponse struct {
	ID             string              `json:"id"`
	Status         string              `json:"status"`
	TotalCents     int64               `json:"total_cents"`
	Currency       string              `json:"currency"`
	Items          []OrderItemResponse `json:"items"`
	FailureCode    string              `json:"failure_code,omitempty"`
	FailureMessage string              `json:"failure_message,omitempty"`
	// Refund is present only on a cancelled order whose payment went through: "pending" while the
	// money is on its way back, "refunded" once it is.
	Refund          string             `json:"refund,omitempty"`
	ShippingAddress ShippingAddressDTO `json:"shipping_address"`
	Carrier         string             `json:"carrier,omitempty"`
	TrackingCode    string             `json:"tracking_code,omitempty"`
	ShippedAt       *time.Time         `json:"shipped_at,omitempty"`
	DeliveredAt     *time.Time         `json:"delivered_at,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

func ToOrderResponse(o *domain.Order) OrderResponse {
	items := make([]OrderItemResponse, len(o.Items))
	for i, it := range o.Items {
		items[i] = OrderItemResponse{
			VariantID: it.VariantID.String(), ProductID: it.ProductID.String(), SKU: it.SKU,
			ProductName: it.ProductName, Size: it.Size, Color: it.Color,
			UnitPriceCents: it.UnitPriceCents, Quantity: it.Quantity, LineTotalCents: it.LineTotalCents(),
		}
	}
	return OrderResponse{
		ID: o.ID.String(), Status: string(o.Status), TotalCents: o.TotalCents, Currency: o.Currency,
		Items: items, FailureCode: o.FailureCode, FailureMessage: o.FailureMessage, Refund: refundLabel(o.RefundState),
		ShippingAddress: ShippingAddressDTO{
			FullName: o.ShippingAddress.FullName, Phone: o.ShippingAddress.Phone, Line1: o.ShippingAddress.Line1,
			Line2: o.ShippingAddress.Line2, District: o.ShippingAddress.District, City: o.ShippingAddress.City,
			PostalCode: o.ShippingAddress.PostalCode, Country: o.ShippingAddress.Country,
		},
		Carrier: o.Carrier, TrackingCode: o.TrackingCode, ShippedAt: o.ShippedAt, DeliveredAt: o.DeliveredAt,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

func refundLabel(s domain.RefundState) string {
	switch s {
	case domain.RefundRequested:
		return "pending"
	case domain.RefundRefunded:
		return "refunded"
	}
	return ""
}

// HistoryEntryResponse is what the customer sees: the status and when it was reached.
type HistoryEntryResponse struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}

type OrderDetailResponse struct {
	OrderResponse
	History []HistoryEntryResponse `json:"history"`
}

func ToOrderDetailResponse(d *application.OrderDetail) OrderDetailResponse {
	h := make([]HistoryEntryResponse, 0, len(d.History))
	for _, c := range d.History {
		// Notes that do not change the status (stock released) are for operators.
		if c.FromStatus == nil || *c.FromStatus != c.ToStatus {
			h = append(h, HistoryEntryResponse{Status: string(c.ToStatus), At: c.CreatedAt})
		}
	}
	return OrderDetailResponse{OrderResponse: ToOrderResponse(d.Order), History: h}
}

// AdminOrderResponse adds the internal bookkeeping operators need.
type AdminOrderResponse struct {
	OrderResponse
	CustomerID       string  `json:"customer_id"`
	ReservationID    *string `json:"reservation_id"`
	ReservationState string  `json:"reservation_state"`
	PaymentRequests  int     `json:"payment_requests"`
	RefundState      string  `json:"refund_state"`
	RefundRequests   int     `json:"refund_requests"`
}

func ToAdminOrderResponse(o *domain.Order) AdminOrderResponse {
	r := AdminOrderResponse{
		OrderResponse: ToOrderResponse(o), CustomerID: o.CustomerID.String(),
		ReservationState: string(o.ReservationState), PaymentRequests: o.PaymentRequests,
		RefundState: string(o.RefundState), RefundRequests: o.RefundRequests,
	}
	if o.ReservationID != nil {
		s := o.ReservationID.String()
		r.ReservationID = &s
	}
	return r
}

type AdminHistoryEntryResponse struct {
	From   *string   `json:"from"`
	To     string    `json:"to"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

type AdminOrderDetailResponse struct {
	AdminOrderResponse
	History []AdminHistoryEntryResponse `json:"history"`
}

func ToAdminOrderDetailResponse(d *application.OrderDetail) AdminOrderDetailResponse {
	h := make([]AdminHistoryEntryResponse, len(d.History))
	for i, c := range d.History {
		e := AdminHistoryEntryResponse{To: string(c.ToStatus), Reason: c.Reason, At: c.CreatedAt}
		if c.FromStatus != nil {
			s := string(*c.FromStatus)
			e.From = &s
		}
		h[i] = e
	}
	return AdminOrderDetailResponse{AdminOrderResponse: ToAdminOrderResponse(d.Order), History: h}
}
