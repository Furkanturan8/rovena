package http

import (
	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
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

type UserHandler struct {
	users *application.UserService
}

func NewUserHandler(users *application.UserService) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Me(c *fiber.Ctx) error {
	id, err := currentUserID(c)
	if err != nil {
		return err
	}
	user, err := h.users.Get(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToUserResponse(user))
}

func (h *UserHandler) UpdateMe(c *fiber.Ctx) error {
	id, err := currentUserID(c)
	if err != nil {
		return err
	}
	var req UpdateProfileRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	user, err := h.users.UpdateProfile(c.UserContext(), id, application.UpdateProfileInput{
		FirstName: req.FirstName, LastName: req.LastName,
	})
	if err != nil {
		return err
	}
	return response.OK(c, ToUserResponse(user))
}

func (h *UserHandler) List(c *fiber.Ctx) error {
	page := max(c.QueryInt("page", 1), 1)
	size := min(max(c.QueryInt("page_size", defaultPageSize), 1), maxPageSize)

	users, total, err := h.users.List(c.UserContext(), repository.ListUsersParams{Page: page, PageSize: size})
	if err != nil {
		return err
	}
	items := make([]UserResponse, len(users))
	for i := range users {
		items[i] = ToUserResponse(&users[i])
	}
	return response.OK(c, response.Page[UserResponse]{Items: items, Total: total, Page: page, PageSize: size})
}

func (h *UserHandler) GetByID(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	user, err := h.users.Get(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, ToUserResponse(user))
}

func (h *UserHandler) ChangeRole(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var req ChangeRoleRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	user, err := h.users.ChangeRole(c.UserContext(), id, domain.Role(req.Role))
	if err != nil {
		return err
	}
	return response.OK(c, ToUserResponse(user))
}

func currentUserID(c *fiber.Ctx) (uuid.UUID, error) {
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
