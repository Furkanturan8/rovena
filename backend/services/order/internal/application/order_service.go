package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/telemetry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ErrUnexpectedEvent marks an event that cannot be handled however often it is
// retried (unknown order, impossible state, wrong amount). The consumer sends it to
// the dead-letter queue instead of retrying.
var ErrUnexpectedEvent = errors.New("unexpected event")

// paymentOrderAlreadyPaid is the failure code Payment uses when a second payment for
// an already paid order is refused. It says nothing about the order being cancelled.
const paymentOrderAlreadyPaid = "order_already_paid"

const (
	maxIdempotencyKeyLen = 64
	maxPaymentMethodLen  = 128
)

type Config struct {
	// StuckAfter is how long an order may sit unchanged before the recovery job looks at it.
	StuckAfter time.Duration
	// PendingMaxAge is how long an order may wait for its stock to be reserved
	// (Inventory unreachable) before it is cancelled.
	PendingMaxAge time.Duration
	// PaymentTimeout is how long to wait for a payment answer before asking again.
	PaymentTimeout time.Duration
	// MaxPaymentRequests bounds how often PaymentRequested is sent for one order. When
	// the last request stays unanswered for PaymentTimeout, the order is cancelled.
	MaxPaymentRequests int
	// RecoveryBatch is how many orders one recovery pass handles at most.
	RecoveryBatch int
}

func (c Config) withDefaults() Config {
	if c.StuckAfter <= 0 {
		c.StuckAfter = 30 * time.Second
	}
	if c.PendingMaxAge <= 0 {
		c.PendingMaxAge = 5 * time.Minute
	}
	if c.PaymentTimeout <= 0 {
		c.PaymentTimeout = 2 * time.Minute
	}
	if c.MaxPaymentRequests <= 0 {
		c.MaxPaymentRequests = 3
	}
	if c.RecoveryBatch <= 0 {
		c.RecoveryBatch = 50
	}
	return c
}

var (
	refundsRequested = telemetry.Counter("orders.refunds.requested", "Refunds asked of Payment for cancelled orders, by reason")
	ordersFinished   = telemetry.Counter("orders.finished", "Orders that reached a final status, by status and cancellation code")
	recoveryActions  = telemetry.Counter("saga.recovery.actions", "Repairs made by the recovery job, by action")
)

func countRecovery(ctx context.Context, action string) {
	recoveryActions.Add(ctx, 1, metric.WithAttributes(attribute.String("action", action)))
}

// OrderService runs the checkout saga: it places orders, reacts to payment outcomes
// and repairs orders that got stuck between two steps.
type OrderService struct {
	repo      repository.OrderRepository
	catalog   Catalog
	inventory Inventory
	log       *slog.Logger
	cfg       Config
	now       func() time.Time
}

func NewOrderService(repo repository.OrderRepository, catalog Catalog, inventory Inventory, log *slog.Logger, cfg Config) *OrderService {
	if log == nil {
		log = slog.Default()
	}
	return &OrderService{repo: repo, catalog: catalog, inventory: inventory, log: log, cfg: cfg.withDefaults(), now: time.Now}
}

type CheckoutInput struct {
	CustomerID     uuid.UUID
	IdempotencyKey string
	PaymentMethod  string
	Shipping       domain.ShippingAddress
}

// Checkout turns the customer's cart into an order and starts the saga. created is
// false when the idempotency key was already used, which makes a retried request safe.
//
// The returned order may still be pending: when Inventory cannot answer, the saga
// continues in the background (see Recover) instead of failing the customer's request.
func (s *OrderService) Checkout(ctx context.Context, in CheckoutInput) (order *domain.Order, created bool, err error) {
	if err := validateCheckout(in); err != nil {
		return nil, false, err
	}
	if o, err := s.existing(ctx, in); o != nil || err != nil {
		return o, false, err
	}

	cart, err := s.repo.GetCart(ctx, in.CustomerID)
	if err != nil {
		return nil, false, err
	}
	if len(cart) == 0 {
		// Either the cart really is empty, or a request with the same key emptied it a moment ago
		// (we passed the key check just before that request committed). The key decides which.
		if prior, perr := s.existing(ctx, in); prior != nil || perr != nil {
			return prior, false, perr
		}
		return nil, false, domain.ErrCartEmpty
	}
	items, total, currency, err := s.price(ctx, cart)
	if err != nil {
		return nil, false, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, false, err
	}
	now := s.now()
	o := &domain.Order{
		ID: id, CustomerID: in.CustomerID, Status: domain.StatusPending,
		TotalCents: total, Currency: currency, IdempotencyKey: in.IdempotencyKey,
		PaymentMethod: in.PaymentMethod, ShippingAddress: in.Shipping, ReservationState: domain.ReservationNone, RefundState: domain.RefundNone,
		Items: items, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, o, cart); err != nil {
		// Two requests with the same key raced, or the first one already emptied the cart.
		if errors.Is(err, domain.ErrDuplicateCheckout) || errors.Is(err, domain.ErrCartChanged) {
			if prior, perr := s.existing(ctx, in); prior != nil || perr != nil {
				return prior, false, perr
			}
		}
		return nil, false, err
	}

	s.advance(ctx, o)
	fresh, err := s.repo.GetByID(ctx, o.ID)
	return fresh, true, err
}

func validateCheckout(in CheckoutInput) error {
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > maxIdempotencyKeyLen {
		return fmt.Errorf("%w: idempotency key must be 1-%d characters", domain.ErrInvalidRequest, maxIdempotencyKeyLen)
	}
	for _, r := range in.IdempotencyKey {
		if r < 0x21 || r > 0x7e {
			return fmt.Errorf("%w: idempotency key must be printable ASCII without spaces", domain.ErrInvalidRequest)
		}
	}
	if in.PaymentMethod == "" || len(in.PaymentMethod) > maxPaymentMethodLen {
		return fmt.Errorf("%w: payment method must be 1-%d characters", domain.ErrInvalidRequest, maxPaymentMethodLen)
	}
	a := in.Shipping
	if a.FullName == "" || a.Phone == "" || a.Line1 == "" || a.District == "" || a.City == "" {
		return fmt.Errorf("%w: a complete shipping address is required", domain.ErrInvalidRequest)
	}
	return nil
}

// existing returns the order a previous request with the same key created, finishing
// its first step if that request died half way. nil, nil means the key is new.
func (s *OrderService) existing(ctx context.Context, in CheckoutInput) (*domain.Order, error) {
	o, err := s.repo.GetByIdempotencyKey(ctx, in.CustomerID, in.IdempotencyKey)
	if errors.Is(err, domain.ErrOrderNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if o.PaymentMethod != in.PaymentMethod {
		return nil, domain.ErrIdempotencyKeyReused
	}
	if o.Status == domain.StatusPending {
		s.advance(ctx, o)
		return s.repo.GetByID(ctx, o.ID)
	}
	return o, nil
}

// price snapshots the cart with Catalog's current data. Any line that can no longer
// be sold stops the checkout and is named in the error.
func (s *OrderService) price(ctx context.Context, cart []domain.CartItem) ([]domain.OrderItem, int64, string, error) {
	infos, err := lookupAll(ctx, s.catalog, cart)
	if err != nil {
		return nil, 0, "", err
	}
	items := make([]domain.OrderItem, 0, len(cart))
	var total int64
	currency := ""
	for _, c := range cart {
		info, ok := infos[c.VariantID]
		if !ok || !info.Sellable {
			return nil, 0, "", &domain.ItemError{VariantID: c.VariantID, Err: domain.ErrItemUnavailable}
		}
		if currency == "" {
			currency = info.Currency
		} else if currency != info.Currency {
			return nil, 0, "", domain.ErrMixedCurrencies
		}
		items = append(items, domain.OrderItem{
			VariantID: c.VariantID, ProductID: info.ProductID, SKU: info.SKU, ProductName: info.ProductName,
			Size: info.Size, Color: info.Color, UnitPriceCents: info.PriceCents, Quantity: c.Quantity,
		})
		total += info.PriceCents * int64(c.Quantity)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].VariantID.String() < items[j].VariantID.String() })
	return items, total, currency, nil
}

// advance performs the first saga step for a pending order: hold the stock, then queue
// PaymentRequested. It never returns an error: if Inventory is unreachable the order
// stays pending and the recovery job retries; Reserve is idempotent per order.
func (s *OrderService) advance(ctx context.Context, o *domain.Order) {
	lines := make([]ReservationLine, len(o.Items))
	for i, it := range o.Items {
		lines[i] = ReservationLine{VariantID: it.VariantID, Quantity: it.Quantity}
	}

	res, err := s.inventory.Reserve(ctx, o.ID, lines)
	var shortage *StockShortage
	if errors.As(err, &shortage) {
		code, msg := domain.CodeOutOfStock, fmt.Sprintf("Not enough stock for variant %s", shortage.VariantID)
		if shortage.NotTracked {
			code, msg = domain.CodeItemUnavailable, fmt.Sprintf("Variant %s can not be ordered right now", shortage.VariantID)
		}
		s.cancel(ctx, o, []domain.Status{domain.StatusPending}, code, msg)
		return
	}
	if err != nil {
		s.log.WarnContext(ctx, "stock reservation failed, order stays pending", "order_id", o.ID, "error", err)
		return
	}

	ev, err := s.paymentRequested(ctx, o)
	if err != nil {
		s.log.ErrorContext(ctx, "build PaymentRequested", "order_id", o.ID, "error", err)
		return
	}
	changed, err := s.repo.MarkReserved(ctx, o.ID, res.ID, ev)
	if err != nil {
		s.log.WarnContext(ctx, "recording reservation failed, order stays pending", "order_id", o.ID, "error", err)
		return
	}
	if !changed {
		s.releaseIfOrphaned(ctx, o.ID, res.ID)
	}
}

// releaseIfOrphaned frees a reservation that was made for an order that got cancelled
// while the reservation call was in flight: the order never learned about it, so the
// recovery job would not release it either.
func (s *OrderService) releaseIfOrphaned(ctx context.Context, orderID, reservationID uuid.UUID) {
	cur, err := s.repo.GetByID(ctx, orderID)
	if err != nil || cur.Status != domain.StatusCancelled || cur.ReservationState == domain.ReservationHeld {
		return
	}
	if err := s.inventory.Release(ctx, reservationID); err != nil {
		s.log.WarnContext(ctx, "releasing orphaned reservation failed; it will expire on its own",
			"order_id", orderID, "reservation_id", reservationID, "error", err)
	}
}

func (s *OrderService) cancel(ctx context.Context, o *domain.Order, from []domain.Status, code, message string) bool {
	ev, err := s.event(ctx, o, events.OrderCancelled, events.OrderCancelledVersion, events.OrderCancelledPayload{
		OrderID: o.ID, CustomerID: o.CustomerID, Code: code, Message: message,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "build OrderCancelled", "order_id", o.ID, "error", err)
		return false
	}
	changed, err := s.repo.Cancel(ctx, o.ID, from, code, message, ev)
	if err != nil {
		s.log.ErrorContext(ctx, "cancel order", "order_id", o.ID, "error", err)
		return false
	}
	if changed {
		ordersFinished.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "cancelled"), attribute.String("code", code)))
		s.log.InfoContext(ctx, "order cancelled", "order_id", o.ID, "code", code)
		if code != domain.CodeCancelledByCustomer {
			s.restoreCart(ctx, o)
		}
	}
	return changed
}

// restoreCart puts the items of an order that was cancelled for the customer (no stock, a declined
// card, a timeout) back into their cart, so they can fix the problem and try again instead of
// finding the cart empty. Best effort: the order is already cancelled either way.
func (s *OrderService) restoreCart(ctx context.Context, o *domain.Order) {
	items := make([]domain.CartItem, len(o.Items))
	for i, it := range o.Items {
		items[i] = domain.CartItem{VariantID: it.VariantID, Quantity: it.Quantity}
	}
	if err := s.repo.RestoreCart(ctx, o.CustomerID, items); err != nil {
		s.log.WarnContext(ctx, "could not put the items of a cancelled order back into the cart", "order_id", o.ID, "error", err)
	}
}

// requestRefund asks Payment to give back the money of a cancelled order whose payment went
// through anyway (it arrived after the cancellation, or the stock was gone by the time it did).
// Safe to repeat: only the first call for an order queues the request.
func (s *OrderService) requestRefund(ctx context.Context, o *domain.Order, reason string) error {
	ev, err := s.event(ctx, o, events.RefundRequested, events.RefundRequestedVersion, events.RefundRequestedPayload{
		OrderID: o.ID, CustomerID: o.CustomerID, Reason: reason,
	})
	if err != nil {
		return err
	}
	changed, err := s.repo.RequestRefund(ctx, o.ID, ev)
	if err != nil {
		return err
	}
	if changed {
		refundsRequested.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
		s.log.WarnContext(ctx, "money was taken for a cancelled order, refund requested",
			"order_id", o.ID, "reason", reason, "amount_cents", o.TotalCents)
	}
	return nil
}

// CancelByCustomer lets the customer cancel an order that is not paid yet. Cancelling while the
// payment is in flight is allowed on purpose: if the payment succeeds anyway, the order is
// already cancelled, and the money is refunded like for any other late payment.
// It is idempotent, and an order cancelled for another reason is returned as it is.
func (s *OrderService) CancelByCustomer(ctx context.Context, orderID, customerID uuid.UUID) (*domain.Order, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if o.CustomerID != customerID {
		return nil, domain.ErrOrderNotFound
	}
	switch o.Status {
	case domain.StatusCancelled:
		return o, nil
	case domain.StatusPaid, domain.StatusConfirmed, domain.StatusShipped, domain.StatusDelivered:
		return nil, domain.ErrCannotCancel
	}

	s.cancel(ctx, o, []domain.Status{domain.StatusPending, domain.StatusAwaitingPayment},
		domain.CodeCancelledByCustomer, "You cancelled this order")
	fresh, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	switch fresh.Status {
	case domain.StatusCancelled:
		// Releasing the stock is not the customer's concern; if Inventory is down the recovery
		// job finishes it.
		if err := s.settle(ctx, fresh); err != nil {
			s.log.WarnContext(ctx, "releasing stock of a cancelled order failed, recovery will retry", "order_id", orderID, "error", err)
		}
		return s.repo.GetByID(ctx, orderID)
	default:
		return nil, domain.ErrCannotCancel // the payment arrived while we were cancelling
	}
}

func (s *OrderService) paymentRequested(ctx context.Context, o *domain.Order) (messaging.Envelope, error) {
	return s.event(ctx, o, events.PaymentRequested, events.PaymentRequestedVersion, events.PaymentRequestedPayload{
		OrderID: o.ID, CustomerID: o.CustomerID, IdempotencyKey: o.PaymentIdempotencyKey(),
		AmountCents: o.TotalCents, Currency: o.Currency, PaymentMethod: o.PaymentMethod,
	})
}

// event builds an envelope whose correlation id is the order id, so every event and log
// line of one checkout can be found together.
func (s *OrderService) event(ctx context.Context, o *domain.Order, eventType string, version int, payload any) (messaging.Envelope, error) {
	return messaging.NewEnvelope(messaging.WithCorrelationID(ctx, o.ID.String()), eventType, version, payload)
}

// Ship hands a confirmed order to the carrier. Repeating it with the same tracking code is harmless.
func (s *OrderService) Ship(ctx context.Context, id uuid.UUID, carrier, trackingCode string) (*domain.Order, error) {
	if carrier == "" || len(carrier) > 60 || trackingCode == "" || len(trackingCode) > 60 {
		return nil, fmt.Errorf("%w: carrier and tracking code are required (at most 60 characters)", domain.ErrInvalidRequest)
	}
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o.Status == domain.StatusShipped && o.Carrier == carrier && o.TrackingCode == trackingCode {
		return o, nil
	}
	changed, err := s.repo.Ship(ctx, id, carrier, trackingCode)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, domain.ErrCannotShip
	}
	return s.repo.GetByID(ctx, id)
}

// Deliver marks a shipped order as delivered. Repeating it is harmless.
func (s *OrderService) Deliver(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o.Status == domain.StatusDelivered {
		return o, nil
	}
	changed, err := s.repo.Deliver(ctx, id)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, domain.ErrCannotDeliver
	}
	return s.repo.GetByID(ctx, id)
}
