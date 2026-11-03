package handler_auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/response"
	"github.com/SarawinT/user-management-api/middleware"
)

// Me returns the user the access token belongs to. Requires a JWT.
func (h *AuthHandler) Me(c fiber.Ctx) error {
	claims, ok := middleware.ClaimsFrom(c)
	if !ok {
		return entity.ErrInvalidToken
	}

	u, err := h.userService.InquiryUser(c.Context(), claims.UserID)
	if err != nil {
		return err
	}
	return response.SendSuccessResponse(c, fiber.StatusOK, response.NewUserResponse(u))
}
