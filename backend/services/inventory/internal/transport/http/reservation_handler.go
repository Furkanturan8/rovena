package http

import (
	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ReservationHandler struct {
	reservations *application.ReservationService
}

func NewReservationHandler(reservations *application.ReservationService) *ReservationHandler {
	return &ReservationHandler{reservations: reservations}
}

// Create holds stock for an order. It answers 201 for a new reservation and 200
// when the order already had one, so the caller can safely retry.
func (h *ReservationHandler) Create(c *fiber.Ctx) error {
	var req CreateReservationRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	orderID, _ := uuid.Parse(req.OrderID) // format already validated
	items := make([]domain.ReservationItem, len(req.Items))
	for i, it := range req.Items {
		variantID, _ := uuid.Parse(it.VariantID)
		items[i] = domain.ReservationItem{VariantID: variantID, Quantity: it.Quantity}
	}

	res, created, err := h.reservations.Reserve(c.UserContext(), application.ReserveInput{
		OrderID: orderID, Items: items,
	})
	if err != nil {
		return err
	}
	if !created {
		return response.OK(c, ToReservationResponse(res))
	}
	return response.Created(c, ToReservationResponse(res))
}

func (h *ReservationHandler) Get(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	res, err := h.reservations.Get(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToReservationResponse(res))
}

func (h *ReservationHandler) GetByOrder(c *fiber.Ctx) error {
	orderID, err := pathUUID(c, "orderId")
	if err != nil {
		return err
	}
	res, err := h.reservations.GetByOrder(c.UserContext(), orderID)
	if err != nil {
		return err
	}
	return response.OK(c, ToReservationResponse(res))
}

// Commit turns the hold into a sale. Repeating it on an already committed
// reservation succeeds without moving stock a second time.
func (h *ReservationHandler) Commit(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	res, err := h.reservations.Commit(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToReservationResponse(res))
}

// Release is the compensating action of the checkout saga: the order failed, so
// the held stock goes back on sale. It is idempotent as well.
func (h *ReservationHandler) Release(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	res, err := h.reservations.Release(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToReservationResponse(res))
}
