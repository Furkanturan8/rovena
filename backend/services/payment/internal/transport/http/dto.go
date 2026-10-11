package http

import (
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
)

// PaymentResponse is what a customer sees about their own payment.
type PaymentResponse struct {
	ID             string     `json:"id"`
	OrderID        string     `json:"order_id"`
	Status         string     `json:"status"`
	AmountCents    int64      `json:"amount_cents"`
	Currency       string     `json:"currency"`
	FailureCode    string     `json:"failure_code"`
	FailureMessage string     `json:"failure_message"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

func ToPaymentResponse(p *domain.Payment) PaymentResponse {
	return PaymentResponse{
		ID: p.ID.String(), OrderID: p.OrderID.String(), Status: string(p.Status),
		AmountCents: p.AmountCents, Currency: p.Currency,
		FailureCode: p.FailureCode, FailureMessage: p.FailureMessage,
		CreatedAt: p.CreatedAt, CompletedAt: p.CompletedAt,
	}
}

// AdminPaymentResponse adds the operational detail that only staff and other services
// need: who paid, which provider reference to look up, how many attempts it took.
type AdminPaymentResponse struct {
	PaymentResponse
	CustomerID     string     `json:"customer_id"`
	IdempotencyKey string     `json:"idempotency_key"`
	PaymentMethod  string     `json:"payment_method"`
	Attempts       int        `json:"attempts"`
	ProviderRef    string     `json:"provider_ref"`
	RefundRef      string     `json:"refund_ref"`
	RefundedAt     *time.Time `json:"refunded_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func ToAdminPaymentResponse(p *domain.Payment) AdminPaymentResponse {
	return AdminPaymentResponse{
		PaymentResponse: ToPaymentResponse(p),
		CustomerID:      p.CustomerID.String(),
		IdempotencyKey:  p.IdempotencyKey,
		PaymentMethod:   p.PaymentMethod,
		Attempts:        p.Attempts,
		ProviderRef:     p.ProviderRef,
		RefundRef:       p.RefundRef,
		RefundedAt:      p.RefundedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}
