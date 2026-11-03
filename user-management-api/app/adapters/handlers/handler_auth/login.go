package handler_auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/models/response"
)

// Login checks email and password and returns a JWT access token. Public.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req request.LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON request body")
	}

	res, err := h.authService.Login(c.Context(), req)
	if err != nil {
		return err
	}

	c.Set(fiber.HeaderCacheControl, "no-store") // never cache tokens
	return response.SendSuccessResponse(c, fiber.StatusOK, response.NewLoginResponse(res))
}
