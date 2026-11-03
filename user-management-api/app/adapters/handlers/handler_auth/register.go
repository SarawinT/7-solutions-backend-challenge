package handler_auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/models/response"
)

// Register creates a new account. Public.
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req request.RegisterRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON request body")
	}

	u, err := h.authService.Register(c.Context(), req)
	if err != nil {
		return err
	}

	c.Location("/api/v1/users/" + u.ID)
	return response.SendSuccessResponse(c, fiber.StatusCreated, response.NewUserResponse(u))
}
