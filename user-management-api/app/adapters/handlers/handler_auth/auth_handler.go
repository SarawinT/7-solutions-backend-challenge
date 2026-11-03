// Package handler_auth holds the REST handlers for /api/v1/auth.
package handler_auth

import (
	"github.com/SarawinT/user-management-api/app/core/services/service_auth"
	"github.com/SarawinT/user-management-api/app/core/services/service_user"
)

type AuthHandler struct {
	authService service_auth.AuthServicer
	userService service_user.UserServicer
}

func NewAuthHandler(authService service_auth.AuthServicer, userService service_user.UserServicer) *AuthHandler {
	return &AuthHandler{authService: authService, userService: userService}
}
