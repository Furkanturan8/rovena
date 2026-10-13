package http

import (
	"strconv"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
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

type ProductHandler struct {
	products *application.ProductService
}

func NewProductHandler(products *application.ProductService) *ProductHandler {
	return &ProductHandler{products: products}
}

// ---- public ----

func (h *ProductHandler) List(c *fiber.Ctx) error {
	f, err := parseFilter(c)
	if err != nil {
		return err
	}
	items, total, err := h.products.ListPublic(c.UserContext(), f)
	if err != nil {
		return err
	}
	publicCache(c)
	return response.OK(c, summaryPage(items, total, f))
}

func (h *ProductHandler) Get(c *fiber.Ctx) error {
	p, err := h.products.GetPublic(c.UserContext(), c.Params("idOrSlug"))
	if err != nil {
		return err
	}
	publicCache(c)
	return response.OK(c, ToProductResponse(p))
}

// ---- admin ----

func (h *ProductHandler) AdminList(c *fiber.Ctx) error {
	f, err := parseFilter(c)
	if err != nil {
		return err
	}
	if s := c.Query("status"); s != "" {
		st := domain.ProductStatus(s)
		if !st.Valid() {
			return errorx.WithMessage(errorx.ErrInvalidRequest, "Invalid status filter")
		}
		f.Status = &st
	}
	items, total, err := h.products.ListAdmin(c.UserContext(), f)
	if err != nil {
		return err
	}
	return response.OK(c, summaryPage(items, total, f))
}

func (h *ProductHandler) AdminGet(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	p, err := h.products.GetAdmin(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToProductResponse(p))
}

func (h *ProductHandler) Create(c *fiber.Ctx) error {
	var req CreateProductRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	categoryID, _ := uuid.Parse(req.CategoryID) // format already validated
	in := application.CreateProductInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description, Brand: req.Brand, CategoryID: categoryID,
		Status: domain.ProductStatus(req.Status), Images: toImages(req.Images),
		Translations: req.Translations, Attributes: req.Attributes,
	}
	for _, v := range req.Variants {
		in.Variants = append(in.Variants, v.toInput())
	}
	p, err := h.products.Create(c.UserContext(), in)
	if err != nil {
		return err
	}
	return response.Created(c, ToProductResponse(p))
}

func (h *ProductHandler) Update(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var req UpdateProductRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	in := application.UpdateProductInput{
		Name: req.Name, Description: req.Description, Brand: req.Brand,
		Translations: req.Translations, Attributes: req.Attributes,
	}
	if req.CategoryID != nil {
		cid, _ := uuid.Parse(*req.CategoryID)
		in.CategoryID = &cid
	}
	if req.Status != nil {
		st := domain.ProductStatus(*req.Status)
		in.Status = &st
	}
	if req.Images != nil {
		imgs := toImages(*req.Images)
		in.Images = &imgs
	}
	p, err := h.products.Update(c.UserContext(), id, in)
	if err != nil {
		return err
	}
	return response.OK(c, ToProductResponse(p))
}

func (h *ProductHandler) Archive(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.products.Archive(c.UserContext(), id); err != nil {
		return err
	}
	return response.NoContent(c)
}

func (h *ProductHandler) AddVariant(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var req VariantRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	v, err := h.products.AddVariant(c.UserContext(), id, req.toInput())
	if err != nil {
		return err
	}
	return response.Created(c, ToVariantResponse(v))
}

func (h *ProductHandler) UpdateVariant(c *fiber.Ctx) error {
	productID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	variantID, err := pathUUID(c, "variantId")
	if err != nil {
		return err
	}
	var req UpdateVariantRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	v, err := h.products.UpdateVariant(c.UserContext(), productID, variantID, application.UpdateVariantInput{
		Size: req.Size, Color: req.Color, PriceCents: req.PriceCents, Active: req.Active,
	})
	if err != nil {
		return err
	}
	return response.OK(c, ToVariantResponse(v))
}

// ---- helpers ----

func parseFilter(c *fiber.Ctx) (domain.ProductFilter, error) {
	page, size := httpx.Pagination(c, defaultPageSize, maxPageSize)
	f := domain.ProductFilter{
		Query:        c.Query("q"),
		CategorySlug: c.Query("category"),
		Size:         c.Query("size"),
		Color:        c.Query("color"),
		Sort:         domain.ProductSort(c.Query("sort", string(domain.SortNewest))),
		Page:         page,
		PageSize:     size,
	}
	if len(f.Query) > 100 {
		return f, errorx.WithMessage(errorx.ErrInvalidRequest, "Search query is too long")
	}
	if !f.Sort.Valid() {
		return f, errorx.WithMessage(errorx.ErrInvalidRequest, "sort must be one of: newest, price_asc, price_desc, name")
	}
	var err error
	if f.MinPrice, err = optionalPrice(c, "min_price"); err != nil {
		return f, err
	}
	if f.MaxPrice, err = optionalPrice(c, "max_price"); err != nil {
		return f, err
	}
	if f.MinPrice != nil && f.MaxPrice != nil && *f.MinPrice > *f.MaxPrice {
		return f, errorx.WithMessage(errorx.ErrInvalidRequest, "min_price must not exceed max_price")
	}
	return f, nil
}

func optionalPrice(c *fiber.Ctx, name string) (*int64, error) {
	raw := c.Query(name)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return nil, errorx.WithMessage(errorx.ErrInvalidRequest, name+" must be a non-negative integer (minor units)")
	}
	return &n, nil
}

func summaryPage(items []domain.ProductSummary, total int, f domain.ProductFilter) response.Page[ProductSummaryResponse] {
	out := make([]ProductSummaryResponse, len(items))
	for i, s := range items {
		out[i] = ToSummaryResponse(s)
	}
	return response.Page[ProductSummaryResponse]{Items: out, Total: total, Page: f.Page, PageSize: f.PageSize}
}

// LookupVariants returns current price and name for the given variants. Ids that do
// not exist are left out, so the caller can tell which lines vanished.
func (h *ProductHandler) LookupVariants(c *fiber.Ctx) error {
	var req LookupVariantsRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(req.VariantIDs))
	for i, s := range req.VariantIDs {
		ids[i], _ = uuid.Parse(s) // format already validated
	}
	snaps, err := h.products.VariantSnapshots(c.UserContext(), ids)
	if err != nil {
		return err
	}
	out := make([]VariantSnapshotResponse, len(snaps))
	for i, s := range snaps {
		out[i] = ToVariantSnapshotResponse(s)
	}
	return response.OK(c, out)
}
