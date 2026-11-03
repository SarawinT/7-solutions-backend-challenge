package handler_user

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/models/response"
)

// InquiryUser returns one user by id.
func (h *UserHandler) InquiryUser(c fiber.Ctx) error {
	u, err := h.userService.InquiryUser(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.SendSuccessResponse(c, fiber.StatusOK, response.NewUserResponse(u))
}
