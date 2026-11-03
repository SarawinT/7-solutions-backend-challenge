package handler_user

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/models/response"
)

// DeleteUser deletes a user. Responds with a success envelope and no data.
func (h *UserHandler) DeleteUser(c fiber.Ctx) error {
	if err := h.userService.DeleteUser(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.SendSuccessResponse(c, fiber.StatusOK, nil)
}
