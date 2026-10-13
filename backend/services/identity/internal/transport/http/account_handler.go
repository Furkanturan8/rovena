package http

import (
	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/response"
	"github.com/gofiber/fiber/v2"
)

// AccountHandler serves the signed-in customer's own sign-in details and address book.
type AccountHandler struct {
	auth      *application.AuthService
	addresses *application.AddressService
}

func NewAccountHandler(auth *application.AuthService, addresses *application.AddressService) *AccountHandler {
	return &AccountHandler{auth: auth, addresses: addresses}
}

func (h *AccountHandler) ChangePassword(c *fiber.Ctx) error {
	id, err := currentUserID(c)
	if err != nil {
		return err
	}
	var req ChangePasswordRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	pair, err := h.auth.ChangePassword(c.UserContext(), id, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return err
	}
	return response.OK(c, ToTokensResponse(*pair))
}

func (h *AccountHandler) ChangeEmail(c *fiber.Ctx) error {
	id, err := currentUserID(c)
	if err != nil {
		return err
	}
	var req ChangeEmailRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	user, err := h.auth.ChangeEmail(c.UserContext(), id, req.Email, req.Password)
	if err != nil {
		return err
	}
	return response.OK(c, ToUserResponse(user))
}

func (h *AccountHandler) ListAddresses(c *fiber.Ctx) error {
	id, err := currentUserID(c)
	if err != nil {
		return err
	}
	list, err := h.addresses.List(c.UserContext(), id)
	if err != nil {
		return err
	}
	out := make([]AddressResponse, len(list))
	for i := range list {
		out[i] = ToAddressResponse(&list[i])
	}
	return response.OK(c, out)
}

func (h *AccountHandler) CreateAddress(c *fiber.Ctx) error {
	id, err := currentUserID(c)
	if err != nil {
		return err
	}
	var req CreateAddressRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	a, err := h.addresses.Create(c.UserContext(), id, application.AddressInput{
		Title: req.Title, FullName: req.FullName, Phone: req.Phone, Line1: req.Line1, Line2: req.Line2,
		District: req.District, City: req.City, PostalCode: req.PostalCode, IsDefault: req.IsDefault,
	})
	if err != nil {
		return err
	}
	return response.Created(c, ToAddressResponse(a))
}

func (h *AccountHandler) UpdateAddress(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return err
	}
	addrID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var req UpdateAddressRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	a, err := h.addresses.Update(c.UserContext(), userID, addrID, application.AddressPatch{
		Title: req.Title, FullName: req.FullName, Phone: req.Phone, Line1: req.Line1, Line2: req.Line2,
		District: req.District, City: req.City, PostalCode: req.PostalCode, IsDefault: req.IsDefault,
	})
	if err != nil {
		return err
	}
	return response.OK(c, ToAddressResponse(a))
}

func (h *AccountHandler) DeleteAddress(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return err
	}
	addrID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.addresses.Delete(c.UserContext(), userID, addrID); err != nil {
		return err
	}
	return response.NoContent(c)
}

func (h *AccountHandler) SetDefaultAddress(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return err
	}
	addrID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	a, err := h.addresses.SetDefault(c.UserContext(), userID, addrID)
	if err != nil {
		return err
	}
	return response.OK(c, ToAddressResponse(a))
}
