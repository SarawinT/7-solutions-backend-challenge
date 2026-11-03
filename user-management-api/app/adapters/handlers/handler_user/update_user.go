package handler_user

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/models/response"
)

// UpdateUser partially updates a user's name and/or email.
func (h *UserHandler) UpdateUser(c fiber.Ctx) error {
	var req request.UpdateUserRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON request body")
	}
	req.ID = c.Params("id")

	u, err := h.userService.UpdateUser(c.Context(), req)
	if err != nil {
		return err
	}
	return response.SendSuccessResponse(c, fiber.StatusOK, response.NewUserResponse(u))
}
