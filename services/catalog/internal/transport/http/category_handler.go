package http

import (
	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type CategoryHandler struct {
	categories *application.CategoryService
}

func NewCategoryHandler(categories *application.CategoryService) *CategoryHandler {
	return &CategoryHandler{categories: categories}
}

func (h *CategoryHandler) List(c *fiber.Ctx) error {
	cats, err := h.categories.List(c.UserContext())
	if err != nil {
		return err
	}
	items := make([]CategoryResponse, len(cats))
	for i := range cats {
		items[i] = ToCategoryResponse(&cats[i])
	}
	publicCache(c)
	return response.OK(c, items)
}

func (h *CategoryHandler) GetBySlug(c *fiber.Ctx) error {
	cat, err := h.categories.GetBySlug(c.UserContext(), c.Params("slug"))
	if err != nil {
		return err
	}
	publicCache(c)
	return response.OK(c, ToCategoryResponse(cat))
}

func (h *CategoryHandler) Create(c *fiber.Ctx) error {
	var req CreateCategoryRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	in := application.CreateCategoryInput{Name: req.Name, Description: req.Description}
	if req.ParentID != nil {
		id, _ := uuid.Parse(*req.ParentID) // format already validated
		in.ParentID = &id
	}
	cat, err := h.categories.Create(c.UserContext(), in)
	if err != nil {
		return err
	}
	return response.Created(c, ToCategoryResponse(cat))
}

func (h *CategoryHandler) Update(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var req UpdateCategoryRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	cat, err := h.categories.Update(c.UserContext(), id, application.UpdateCategoryInput{
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		return err
	}
	return response.OK(c, ToCategoryResponse(cat))
}

func (h *CategoryHandler) Delete(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.categories.Delete(c.UserContext(), id); err != nil {
		return err
	}
	return response.NoContent(c)
}

func pathUUID(c *fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.Nil, errorx.WithMessage(errorx.ErrInvalidRequest, "Invalid "+name)
	}
	return id, nil
}

// publicCache lets the gateway and browsers cache anonymous catalog reads briefly.
func publicCache(c *fiber.Ctx) {
	c.Set(fiber.HeaderCacheControl, "public, max-age=60")
}
