// Package routers wires each feature (repository → service → handler) and
// registers its routes.
package routers

import (
	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/SarawinT/user-management-api/app/adapters/handlers/handler_auth"
	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb"
	"github.com/SarawinT/user-management-api/app/adapters/security"
	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/core/services/service_auth"
	"github.com/SarawinT/user-management-api/app/core/services/service_user"
	"github.com/SarawinT/user-management-api/middleware"
)

// AuthRouter mounts /api/v1/auth. register and login are public; me needs
// a valid JWT.
func AuthRouter(app *fiber.App, m *middleware.Middleware, db *mongo.Database, token ports.TokenPort) {
	userRepo := mongodb.NewUserRepository(db)

	authHandler := handler_auth.NewAuthHandler(
		service_auth.NewAuthService(userRepo, security.NewBcryptHasher(), security.UUIDGenerator{}, token),
		service_user.NewUserService(userRepo),
	)

	authGroup := app.Group("/api/v1/auth")

	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)

	authGroup.Get("/me", m.ValidateJwt(), authHandler.Me) // middleware runs first
}
