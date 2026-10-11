package http

import (
	"strings"

	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	// maxAvailabilityIDs keeps one request bounded; a product page asks for the
	// variants of a single product.
	maxAvailabilityIDs = 100
)

type StockHandler struct {
	stock *application.StockService
}

func NewStockHandler(stock *application.StockService) *StockHandler {
	return &StockHandler{stock: stock}
}

// ---- public ----

// Availability answers "can I buy this?" for a set of variants.
func (h *StockHandler) Availability(c *fiber.Ctx) error {
	ids, err := parseVariantIDs(c.Query("variant_ids"))
	if err != nil {
		return err
	}
	items, err := h.stock.Availability(c.UserContext(), ids)
	if err != nil {
		return err
	}
	out := make([]AvailabilityResponse, len(items))
	for i, a := range items {
		out[i] = ToAvailabilityResponse(a)
	}
	// Stock changes by the second; a cached answer would promise stock that is gone.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return response.OK(c, out)
}

// ---- admin ----

func (h *StockHandler) List(c *fiber.Ctx) error {
	page, size := httpx.Pagination(c, defaultPageSize, maxPageSize)
	f := domain.StockFilter{
		SKU:      c.Query("sku"),
		Sort:     domain.StockSort(c.Query("sort", string(domain.SortNewest))),
		Page:     page,
		PageSize: size,
	}
	if len(f.SKU) > 64 {
		return errorx.WithMessage(errorx.ErrInvalidRequest, "sku filter is too long")
	}
	if !f.Sort.Valid() {
		return errorx.WithMessage(errorx.ErrInvalidRequest, "sort must be one of: newest, sku, available_asc")
	}
	if c.Query("below") != "" {
		below := c.QueryInt("below")
		if below <= 0 {
			return errorx.WithMessage(errorx.ErrInvalidRequest, "below must be a positive integer")
		}
		f.Below = &below
	}

	items, total, err := h.stock.List(c.UserContext(), f)
	if err != nil {
		return err
	}
	out := make([]StockResponse, len(items))
	for i := range items {
		out[i] = ToStockResponse(&items[i])
	}
	return response.OK(c, response.Page[StockResponse]{Items: out, Total: total, Page: f.Page, PageSize: f.PageSize})
}

func (h *StockHandler) Get(c *fiber.Ctx) error {
	variantID, err := pathUUID(c, "variantId")
	if err != nil {
		return err
	}
	item, err := h.stock.Get(c.UserContext(), variantID)
	if err != nil {
		return err
	}
	return response.OK(c, ToStockResponse(item))
}

func (h *StockHandler) Create(c *fiber.Ctx) error {
	var req CreateStockRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	variantID, _ := uuid.Parse(req.VariantID) // format already validated
	item, err := h.stock.Create(c.UserContext(), application.CreateStockInput{
		VariantID: variantID, SKU: req.SKU, OnHand: req.OnHand, Reason: req.Reason,
	})
	if err != nil {
		return err
	}
	return response.Created(c, ToStockResponse(item))
}

func (h *StockHandler) Receive(c *fiber.Ctx) error {
	variantID, err := pathUUID(c, "variantId")
	if err != nil {
		return err
	}
	var req ReceiveStockRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	item, err := h.stock.Receive(c.UserContext(), variantID, req.Quantity, req.Reason)
	if err != nil {
		return err
	}
	return response.OK(c, ToStockResponse(item))
}

func (h *StockHandler) Adjust(c *fiber.Ctx) error {
	variantID, err := pathUUID(c, "variantId")
	if err != nil {
		return err
	}
	var req AdjustStockRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	item, err := h.stock.Adjust(c.UserContext(), variantID, req.Delta, req.Reason)
	if err != nil {
		return err
	}
	return response.OK(c, ToStockResponse(item))
}

func (h *StockHandler) Movements(c *fiber.Ctx) error {
	variantID, err := pathUUID(c, "variantId")
	if err != nil {
		return err
	}
	page, size := httpx.Pagination(c, defaultPageSize, maxPageSize)
	f := domain.MovementFilter{VariantID: variantID, Page: page, PageSize: size}
	items, total, err := h.stock.Movements(c.UserContext(), f)
	if err != nil {
		return err
	}
	out := make([]MovementResponse, len(items))
	for i, m := range items {
		out[i] = ToMovementResponse(m)
	}
	return response.OK(c, response.Page[MovementResponse]{Items: out, Total: total, Page: f.Page, PageSize: f.PageSize})
}

// ---- helpers ----

func parseVariantIDs(raw string) ([]uuid.UUID, error) {
	parts := strings.Split(raw, ",")
	ids := make([]uuid.UUID, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := uuid.Parse(part)
		if err != nil {
			return nil, errorx.WithMessage(errorx.ErrInvalidRequest, "variant_ids must be a comma-separated list of UUIDs")
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errorx.WithMessage(errorx.ErrInvalidRequest, "variant_ids is required")
	}
	if len(ids) > maxAvailabilityIDs {
		return nil, errorx.WithMessage(errorx.ErrInvalidRequest, "variant_ids accepts at most 100 variants")
	}
	return ids, nil
}

func pathUUID(c *fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.Nil, errorx.WithMessage(errorx.ErrInvalidRequest, "Invalid "+name)
	}
	return id, nil
}
