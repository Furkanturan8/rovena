package postgres

import (
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type paymentRow struct {
	bun.BaseModel `bun:"table:payments,alias:p"`

	ID             uuid.UUID  `bun:",pk,type:uuid"`
	OrderID        uuid.UUID  `bun:",type:uuid,notnull"`
	CustomerID     uuid.UUID  `bun:",type:uuid,notnull"`
	IdempotencyKey string     `bun:",notnull"`
	AmountCents    int64      `bun:",notnull"`
	Currency       string     `bun:",notnull"`
	PaymentMethod  string     `bun:",notnull"`
	Status         string     `bun:",notnull"`
	Attempts       int        `bun:",notnull"`
	ProviderRef    string     `bun:",notnull"`
	FailureCode    string     `bun:",notnull"`
	FailureMessage string     `bun:",notnull"`
	CreatedAt      time.Time  `bun:",notnull"`
	UpdatedAt      time.Time  `bun:",notnull"`
	CompletedAt    *time.Time `bun:","`
}

func rowFrom(p *domain.Payment) *paymentRow {
	return &paymentRow{
		ID: p.ID, OrderID: p.OrderID, CustomerID: p.CustomerID, IdempotencyKey: p.IdempotencyKey,
		AmountCents: p.AmountCents, Currency: p.Currency, PaymentMethod: p.PaymentMethod,
		Status: string(p.Status), Attempts: p.Attempts, ProviderRef: p.ProviderRef,
		FailureCode: p.FailureCode, FailureMessage: p.FailureMessage,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, CompletedAt: p.CompletedAt,
	}
}

func (r *paymentRow) toDomain() *domain.Payment {
	return &domain.Payment{
		ID: r.ID, OrderID: r.OrderID, CustomerID: r.CustomerID, IdempotencyKey: r.IdempotencyKey,
		AmountCents: r.AmountCents, Currency: r.Currency, PaymentMethod: r.PaymentMethod,
		Status: domain.Status(r.Status), Attempts: r.Attempts, ProviderRef: r.ProviderRef,
		FailureCode: r.FailureCode, FailureMessage: r.FailureMessage,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, CompletedAt: r.CompletedAt,
	}
}
