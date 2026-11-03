package handler_user

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/models/response"
)

// ListUser returns a page of users (?page=0&pageSize=20).
func (h *UserHandler) ListUser(c fiber.Ctx) error {
	req := request.ListUserRequest{
		Page:     fiber.Query(c, "page", 0),
		PageSize: fiber.Query(c, "pageSize", 0),
	}

	res, err := h.userService.ListUser(c.Context(), req)
	if err != nil {
		return err
	}
	return response.SendSuccessResponse(c, fiber.StatusOK, response.NewListUserResponse(res))
}
