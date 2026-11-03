// Package handler_user holds the REST handlers for /api/v1/users.
package handler_user

import "github.com/SarawinT/user-management-api/app/core/services/service_user"

type UserHandler struct {
	userService service_user.UserServicer
}

func NewUserHandler(userService service_user.UserServicer) *UserHandler {
	return &UserHandler{userService: userService}
}
