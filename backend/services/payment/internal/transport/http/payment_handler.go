package http

import (
	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// PaymentHandler only reads. Payments come into being from PaymentRequested events.
type PaymentHandler struct {
	payments *application.QueryService
}

func NewPaymentHandler(payments *application.QueryService) *PaymentHandler {
	return &PaymentHandler{payments: payments}
}

// MyByOrder returns the caller's latest payment for an order. A payment that belongs
// to someone else answers 404, the same as an order that does not exist.
func (h *PaymentHandler) MyByOrder(c *fiber.Ctx) error {
	orderID, err := pathUUID(c, "orderId")
	if err != nil {
		return err
	}
	claims, ok := auth.FromContext(c)
	if !ok {
		return errorx.ErrUnauthorized
	}
	customerID, err := uuid.Parse(claims.UserID())
	if err != nil {
		return errorx.ErrUnauthorized
	}
	p, err := h.payments.LatestByOrderForCustomer(c.UserContext(), orderID, customerID)
	if err != nil {
		return err
	}
	return response.OK(c, ToPaymentResponse(p))
}

func (h *PaymentHandler) AdminList(c *fiber.Ctx) error {
	page, size := httpx.Pagination(c, defaultPageSize, maxPageSize)
	f := domain.Filter{Status: domain.Status(c.Query("status")), Page: page, PageSize: size}
	if f.Status != "" && !f.Status.Valid() {
		return errorx.WithMessage(errorx.ErrInvalidRequest, "status must be one of: pending, succeeded, failed, refunded")
	}
	if v := c.Query("order_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return errorx.WithMessage(errorx.ErrInvalidRequest, "order_id must be a UUID")
		}
		f.OrderID = &id
	}

	items, total, err := h.payments.List(c.UserContext(), f)
	if err != nil {
		return err
	}
	out := make([]AdminPaymentResponse, len(items))
	for i := range items {
		out[i] = ToAdminPaymentResponse(&items[i])
	}
	return response.OK(c, response.Page[AdminPaymentResponse]{Items: out, Total: total, Page: f.Page, PageSize: f.PageSize})
}

func (h *PaymentHandler) AdminGet(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	p, err := h.payments.Get(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToAdminPaymentResponse(p))
}

// InternalByOrder lets other services ask about an order's payment without a user token.
func (h *PaymentHandler) InternalByOrder(c *fiber.Ctx) error {
	orderID, err := pathUUID(c, "orderId")
	if err != nil {
		return err
	}
	p, err := h.payments.LatestByOrder(c.UserContext(), orderID)
	if err != nil {
		return err
	}
	return response.OK(c, ToAdminPaymentResponse(p))
}

func pathUUID(c *fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.Nil, errorx.WithMessage(errorx.ErrInvalidRequest, "Invalid "+name)
	}
	return id, nil
}
