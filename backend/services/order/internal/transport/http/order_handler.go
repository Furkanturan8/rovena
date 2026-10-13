package http

import (
	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100

	idempotencyKeyHeader = "Idempotency-Key"
)

type OrderHandler struct {
	carts   *application.CartService
	orders  *application.OrderService
	queries *application.QueryService
}

func NewOrderHandler(carts *application.CartService, orders *application.OrderService, queries *application.QueryService) *OrderHandler {
	return &OrderHandler{carts: carts, orders: orders, queries: queries}
}

// ---- cart ----

func (h *OrderHandler) GetCart(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	view, err := h.carts.Get(c.UserContext(), customerID)
	if err != nil {
		return err
	}
	return response.OK(c, ToCartResponse(view))
}

// SetCartItem sets the quantity of a variant. The body carries no price: Catalog is
// the only source of prices.
func (h *OrderHandler) SetCartItem(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	variantID, err := pathUUID(c, "variantId")
	if err != nil {
		return err
	}
	var req SetCartItemRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	view, err := h.carts.SetItem(c.UserContext(), customerID, variantID, req.Quantity)
	if err != nil {
		return err
	}
	return response.OK(c, ToCartResponse(view))
}

func (h *OrderHandler) RemoveCartItem(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	variantID, err := pathUUID(c, "variantId")
	if err != nil {
		return err
	}
	view, err := h.carts.RemoveItem(c.UserContext(), customerID, variantID)
	if err != nil {
		return err
	}
	return response.OK(c, ToCartResponse(view))
}

func (h *OrderHandler) ClearCart(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	if err := h.carts.Clear(c.UserContext(), customerID); err != nil {
		return err
	}
	return response.NoContent(c)
}

// ---- checkout ----

// Checkout places the order. 201 for a new order, 200 when the Idempotency-Key was
// used before and the existing order is returned. The order's status says how far the
// saga got: it is final only once it is confirmed or cancelled.
func (h *OrderHandler) Checkout(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	key := c.Get(idempotencyKeyHeader)
	if key == "" {
		return errorx.WithMessage(errorx.ErrInvalidRequest, "The Idempotency-Key header is required")
	}
	var req CheckoutRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}

	ctx := messaging.WithRequestID(c.UserContext(), httpx.RequestID(c))
	o, created, err := h.orders.Checkout(ctx, application.CheckoutInput{
		CustomerID: customerID, IdempotencyKey: key, PaymentMethod: req.PaymentMethod,
		Shipping: req.ShippingAddress.toDomain(),
	})
	if err != nil {
		return err
	}
	if !created {
		return response.OK(c, ToOrderResponse(o))
	}
	return response.Created(c, ToOrderResponse(o))
}

// ---- customer orders ----

func (h *OrderHandler) MyOrders(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	f, err := parseFilter(c)
	if err != nil {
		return err
	}
	f.CustomerID = &customerID
	items, total, err := h.queries.List(c.UserContext(), f)
	if err != nil {
		return err
	}
	out := make([]OrderResponse, len(items))
	for i := range items {
		out[i] = ToOrderResponse(&items[i])
	}
	return response.OK(c, response.Page[OrderResponse]{Items: out, Total: total, Page: f.Page, PageSize: f.PageSize})
}

func (h *OrderHandler) MyOrder(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	d, err := h.queries.GetForCustomer(c.UserContext(), id, customerID)
	if err != nil {
		return err
	}
	return response.OK(c, ToOrderDetailResponse(d))
}

// Cancel lets the customer cancel an order that is not paid yet. Repeating it is harmless.
func (h *OrderHandler) Cancel(c *fiber.Ctx) error {
	customerID, err := currentCustomer(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	o, err := h.orders.CancelByCustomer(c.UserContext(), id, customerID)
	if err != nil {
		return err
	}
	return response.OK(c, ToOrderResponse(o))
}

// ---- admin ----

func (h *OrderHandler) AdminList(c *fiber.Ctx) error {
	f, err := parseFilter(c)
	if err != nil {
		return err
	}
	if v := c.Query("customer_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return errorx.WithMessage(errorx.ErrInvalidRequest, "customer_id must be a UUID")
		}
		f.CustomerID = &id
	}
	items, total, err := h.queries.List(c.UserContext(), f)
	if err != nil {
		return err
	}
	out := make([]AdminOrderResponse, len(items))
	for i := range items {
		out[i] = ToAdminOrderResponse(&items[i])
	}
	return response.OK(c, response.Page[AdminOrderResponse]{Items: out, Total: total, Page: f.Page, PageSize: f.PageSize})
}

func (h *OrderHandler) AdminGet(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	d, err := h.queries.Get(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToAdminOrderDetailResponse(d))
}

// Ship records the carrier and tracking code of a confirmed order.
func (h *OrderHandler) AdminShip(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var req ShipRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	o, err := h.orders.Ship(c.UserContext(), id, req.Carrier, req.TrackingCode)
	if err != nil {
		return err
	}
	return response.OK(c, ToAdminOrderResponse(o))
}

func (h *OrderHandler) AdminDeliver(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	o, err := h.orders.Deliver(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToAdminOrderResponse(o))
}

// ---- helpers ----

func parseFilter(c *fiber.Ctx) (domain.Filter, error) {
	page, size := httpx.Pagination(c, defaultPageSize, maxPageSize)
	f := domain.Filter{Status: domain.Status(c.Query("status")), Page: page, PageSize: size}
	if f.Status != "" && !f.Status.Valid() {
		return f, errorx.WithMessage(errorx.ErrInvalidRequest,
			"status must be one of: pending, awaiting_payment, paid, confirmed, cancelled")
	}
	return f, nil
}

func currentCustomer(c *fiber.Ctx) (uuid.UUID, error) {
	claims, ok := auth.FromContext(c)
	if !ok {
		return uuid.Nil, errorx.ErrUnauthorized
	}
	id, err := uuid.Parse(claims.UserID())
	if err != nil {
		return uuid.Nil, errorx.ErrUnauthorized
	}
	return id, nil
}

func pathUUID(c *fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.Nil, errorx.WithMessage(errorx.ErrInvalidRequest, "Invalid "+name)
	}
	return id, nil
}
