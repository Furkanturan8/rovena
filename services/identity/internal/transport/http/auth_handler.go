package http

import (
	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/response"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	auth *application.AuthService
}

func NewAuthHandler(auth *application.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	res, err := h.auth.Register(c.UserContext(), req.ToInput())
	if err != nil {
		return err
	}
	return response.Created(c, ToAuthResponse(res))
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	res, err := h.auth.Login(c.UserContext(), req.Email, req.Password)
	if err != nil {
		return err
	}
	return response.OK(c, ToAuthResponse(res))
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	pair, err := h.auth.Refresh(c.UserContext(), req.RefreshToken)
	if err != nil {
		return err
	}
	return response.OK(c, ToTokensResponse(*pair))
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.auth.Logout(c.UserContext(), req.RefreshToken); err != nil {
		return err
	}
	return response.NoContent(c)
}
