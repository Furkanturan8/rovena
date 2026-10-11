package postgres

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/outbox"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type OrderRepository struct {
	db *bun.DB
}

func NewOrderRepository(db *bun.DB) *OrderRepository { return &OrderRepository{db: db} }

var _ repository.OrderRepository = (*OrderRepository)(nil)

// ---- cart ----

func (r *OrderRepository) GetCart(ctx context.Context, customerID uuid.UUID) ([]domain.CartItem, error) {
	return cartItems(ctx, r.db, customerID, false)
}

func cartItems(ctx context.Context, db bun.IDB, customerID uuid.UUID, lock bool) ([]domain.CartItem, error) {
	var rows []cartItemRow
	q := db.NewSelect().Model(&rows).Where("customer_id = ?", customerID).Order("variant_id")
	if lock {
		q = q.For("UPDATE")
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]domain.CartItem, len(rows))
	for i, row := range rows {
		out[i] = domain.CartItem{VariantID: row.VariantID, Quantity: row.Quantity}
	}
	return out, nil
}

// lockCart makes sure the customer's cart row exists and locks it, which serializes
// everything that changes this customer's cart, checkout included.
func lockCart(ctx context.Context, tx bun.Tx, customerID uuid.UUID) error {
	if _, err := tx.NewRaw("INSERT INTO carts (customer_id) VALUES (?) ON CONFLICT DO NOTHING", customerID).Exec(ctx); err != nil {
		return err
	}
	var id uuid.UUID
	return tx.NewSelect().TableExpr("carts").Column("customer_id").
		Where("customer_id = ?", customerID).For("UPDATE").Scan(ctx, &id)
}

func (r *OrderRepository) SetCartItem(ctx context.Context, customerID uuid.UUID, item domain.CartItem) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockCart(ctx, tx, customerID); err != nil {
			return err
		}
		exists, err := tx.NewSelect().Model((*cartItemRow)(nil)).
			Where("customer_id = ? AND variant_id = ?", customerID, item.VariantID).Exists(ctx)
		if err != nil {
			return err
		}
		if !exists {
			n, err := tx.NewSelect().Model((*cartItemRow)(nil)).Where("customer_id = ?", customerID).Count(ctx)
			if err != nil {
				return err
			}
			if n >= domain.MaxCartLines {
				return domain.ErrCartFull
			}
		}
		_, err = tx.NewInsert().Model(&cartItemRow{
			CustomerID: customerID, VariantID: item.VariantID, Quantity: item.Quantity,
		}).On("CONFLICT (customer_id, variant_id) DO UPDATE").Set("quantity = EXCLUDED.quantity").Exec(ctx)
		if err != nil {
			return err
		}
		_, err = tx.NewUpdate().Table("carts").Set("updated_at = now()").Where("customer_id = ?", customerID).Exec(ctx)
		return err
	})
}

func (r *OrderRepository) RemoveCartItem(ctx context.Context, customerID, variantID uuid.UUID) error {
	_, err := r.db.NewDelete().Model((*cartItemRow)(nil)).
		Where("customer_id = ? AND variant_id = ?", customerID, variantID).Exec(ctx)
	return err
}

func (r *OrderRepository) ClearCart(ctx context.Context, customerID uuid.UUID) error {
	_, err := r.db.NewDelete().Model((*cartItemRow)(nil)).Where("customer_id = ?", customerID).Exec(ctx)
	return err
}

// ---- orders ----

func (r *OrderRepository) Create(ctx context.Context, o *domain.Order, cart []domain.CartItem) error {
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockCart(ctx, tx, o.CustomerID); err != nil {
			return err
		}
		current, err := cartItems(ctx, tx, o.CustomerID, true)
		if err != nil {
			return err
		}
		if !sameCart(current, cart) {
			return domain.ErrCartChanged
		}

		if _, err := tx.NewInsert().Model(orderRowFrom(o)).Exec(ctx); err != nil {
			return err
		}
		rows := make([]orderItemRow, len(o.Items))
		for i, it := range o.Items {
			rows[i] = orderItemRow{
				OrderID: o.ID, VariantID: it.VariantID, ProductID: it.ProductID, SKU: it.SKU,
				ProductName: it.ProductName, Size: it.Size, Color: it.Color,
				UnitPriceCents: it.UnitPriceCents, Quantity: it.Quantity,
			}
		}
		if _, err := tx.NewInsert().Model(&rows).Exec(ctx); err != nil {
			return err
		}
		if err := addHistory(ctx, tx, o.ID, nil, o.Status, "order placed"); err != nil {
			return err
		}
		_, err = tx.NewDelete().Model((*cartItemRow)(nil)).Where("customer_id = ?", o.CustomerID).Exec(ctx)
		return err
	})
	return mapConstraint(err)
}

func sameCart(a, b []domain.CartItem) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool { return a[i].VariantID.String() < a[j].VariantID.String() })
	sort.Slice(b, func(i, j int) bool { return b[i].VariantID.String() < b[j].VariantID.String() })
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	return r.getOne(ctx, "o.id = ?", id)
}

func (r *OrderRepository) GetByIdempotencyKey(ctx context.Context, customerID uuid.UUID, key string) (*domain.Order, error) {
	return r.getOne(ctx, "o.customer_id = ? AND o.idempotency_key = ?", customerID, key)
}

func (r *OrderRepository) getOne(ctx context.Context, where string, args ...any) (*domain.Order, error) {
	row := new(orderRow)
	err := r.db.NewSelect().Model(row).Where(where, args...).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	o := row.toDomain()
	orders := []domain.Order{o}
	if err := r.loadItems(ctx, orders); err != nil {
		return nil, err
	}
	return &orders[0], nil
}

func (r *OrderRepository) loadItems(ctx context.Context, orders []domain.Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(orders))
	index := make(map[uuid.UUID]int, len(orders))
	for i, o := range orders {
		ids[i] = o.ID
		index[o.ID] = i
	}
	var rows []orderItemRow
	if err := r.db.NewSelect().Model(&rows).Where("order_id IN (?)", bun.List(ids)).Order("product_name", "sku").Scan(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		i := index[row.OrderID]
		orders[i].Items = append(orders[i].Items, row.toDomain())
	}
	return nil
}

func (r *OrderRepository) History(ctx context.Context, id uuid.UUID) ([]domain.StatusChange, error) {
	var rows []historyRow
	if err := r.db.NewSelect().Model(&rows).Where("order_id = ?", id).Order("id").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]domain.StatusChange, len(rows))
	for i, row := range rows {
		c := domain.StatusChange{ToStatus: domain.Status(row.ToStatus), Reason: row.Reason, CreatedAt: row.CreatedAt}
		if row.FromStatus != nil {
			s := domain.Status(*row.FromStatus)
			c.FromStatus = &s
		}
		out[i] = c
	}
	return out, nil
}

func (r *OrderRepository) List(ctx context.Context, f domain.Filter) ([]domain.Order, int, error) {
	var rows []orderRow
	q := r.db.NewSelect().Model(&rows)
	if f.CustomerID != nil {
		q = q.Where("o.customer_id = ?", *f.CustomerID)
	}
	if f.Status != "" {
		q = q.Where("o.status = ?", string(f.Status))
	}
	total, err := q.Order("o.created_at DESC", "o.id DESC").
		Limit(int64(f.PageSize)).Offset(int64((f.Page - 1) * f.PageSize)).ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	orders := make([]domain.Order, len(rows))
	for i := range rows {
		orders[i] = rows[i].toDomain()
	}
	if err := r.loadItems(ctx, orders); err != nil {
		return nil, 0, err
	}
	return orders, int(total), nil
}

// ---- guarded transitions ----

type transition struct {
	from []domain.Status
	to   domain.Status
	// reservation, when set, is the reservation state the order must be in.
	reservation domain.ReservationState
	reason      string
	set         func(*bun.UpdateQuery) *bun.UpdateQuery
	event       *messaging.Envelope
}

func (r *OrderRepository) apply(ctx context.Context, id uuid.UUID, t transition) (bool, error) {
	changed := false
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var cur struct {
			Status           string
			ReservationState string
		}
		err := tx.NewSelect().TableExpr("orders").Column("status", "reservation_state").
			Where("id = ?", id).For("UPDATE").Scan(ctx, &cur.Status, &cur.ReservationState)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		if !containsStatus(t.from, domain.Status(cur.Status)) {
			return nil
		}
		if t.reservation != "" && domain.ReservationState(cur.ReservationState) != t.reservation {
			return nil
		}

		q := tx.NewUpdate().Table("orders").Set("status = ?", string(t.to)).Set("updated_at = now()").Where("id = ?", id)
		if t.set != nil {
			q = t.set(q)
		}
		if _, err := q.Exec(ctx); err != nil {
			return err
		}
		from := domain.Status(cur.Status)
		if err := addHistory(ctx, tx, id, &from, t.to, t.reason); err != nil {
			return err
		}
		if t.event != nil {
			if err := outbox.Add(ctx, tx, *t.event); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	return changed, err
}

func containsStatus(list []domain.Status, s domain.Status) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func addHistory(ctx context.Context, db bun.IDB, orderID uuid.UUID, from *domain.Status, to domain.Status, reason string) error {
	row := historyRow{OrderID: orderID, ToStatus: string(to), Reason: reason}
	if from != nil {
		s := string(*from)
		row.FromStatus = &s
	}
	_, err := db.NewInsert().Model(&row).Exec(ctx)
	return err
}

func (r *OrderRepository) MarkReserved(ctx context.Context, id, reservationID uuid.UUID, ev messaging.Envelope) (bool, error) {
	return r.apply(ctx, id, transition{
		from: []domain.Status{domain.StatusPending}, to: domain.StatusAwaitingPayment,
		reason: "stock reserved, payment requested", event: &ev,
		set: func(q *bun.UpdateQuery) *bun.UpdateQuery {
			return q.Set("reservation_id = ?", reservationID).
				Set("reservation_state = ?", string(domain.ReservationHeld)).
				Set("payment_requests = payment_requests + 1")
		},
	})
}

func (r *OrderRepository) MarkPaid(ctx context.Context, id uuid.UUID) (bool, error) {
	return r.apply(ctx, id, transition{
		from: []domain.Status{domain.StatusAwaitingPayment}, to: domain.StatusPaid, reason: "payment succeeded",
	})
}

func (r *OrderRepository) Confirm(ctx context.Context, id uuid.UUID, ev messaging.Envelope) (bool, error) {
	return r.apply(ctx, id, transition{
		from: []domain.Status{domain.StatusPaid}, to: domain.StatusConfirmed,
		reservation: domain.ReservationHeld, reason: "stock committed", event: &ev,
		set: func(q *bun.UpdateQuery) *bun.UpdateQuery {
			return q.Set("reservation_state = ?", string(domain.ReservationCommitted))
		},
	})
}

func (r *OrderRepository) Cancel(ctx context.Context, id uuid.UUID, from []domain.Status, code, message string, ev messaging.Envelope) (bool, error) {
	return r.apply(ctx, id, transition{
		from: from, to: domain.StatusCancelled, reason: code, event: &ev,
		set: func(q *bun.UpdateQuery) *bun.UpdateQuery {
			return q.Set("failure_code = ?", code).Set("failure_message = ?", message)
		},
	})
}

func (r *OrderRepository) MarkReleased(ctx context.Context, id uuid.UUID) (bool, error) {
	changed := false
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Table("orders").
			Set("reservation_state = ?", string(domain.ReservationReleased)).Set("updated_at = now()").
			Where("id = ? AND status = ? AND reservation_state = ?", id, string(domain.StatusCancelled), string(domain.ReservationHeld)).
			Exec(ctx)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return nil
		}
		changed = true
		cancelled := domain.StatusCancelled
		return addHistory(ctx, tx, id, &cancelled, cancelled, "stock released")
	})
	return changed, err
}

func (r *OrderRepository) RequestPaymentAgain(ctx context.Context, id uuid.UUID, ev messaging.Envelope) (bool, error) {
	changed := false
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Table("orders").
			Set("payment_requests = payment_requests + 1").Set("updated_at = now()").
			Where("id = ? AND status = ?", id, string(domain.StatusAwaitingPayment)).Exec(ctx)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return nil
		}
		if err := outbox.Add(ctx, tx, ev); err != nil {
			return err
		}
		changed = true
		waiting := domain.StatusAwaitingPayment
		return addHistory(ctx, tx, id, &waiting, waiting, "no payment answer yet, payment requested again")
	})
	return changed, err
}

func (r *OrderRepository) Touch(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.NewUpdate().Table("orders").Set("updated_at = now()").Where("id = ?", id).Exec(ctx)
	return err
}

func (r *OrderRepository) Unfinished(ctx context.Context, olderThan time.Time, limit int) ([]domain.Order, error) {
	var rows []orderRow
	err := r.db.NewSelect().Model(&rows).
		Where("o.updated_at < ?", olderThan).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("o.status IN (?)", bun.List([]string{
				string(domain.StatusPending), string(domain.StatusAwaitingPayment), string(domain.StatusPaid),
			})).WhereOr("o.status = ? AND o.reservation_state = ?", string(domain.StatusCancelled), string(domain.ReservationHeld))
		}).
		Order("o.updated_at").Limit(int64(limit)).Scan(ctx)
	if err != nil {
		return nil, err
	}
	orders := make([]domain.Order, len(rows))
	for i := range rows {
		orders[i] = rows[i].toDomain()
	}
	if err := r.loadItems(ctx, orders); err != nil {
		return nil, err
	}
	return orders, nil
}
