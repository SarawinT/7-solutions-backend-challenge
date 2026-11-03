package routers

import (
	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/SarawinT/user-management-api/app/adapters/handlers/handler_user"
	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb"
	"github.com/SarawinT/user-management-api/app/core/services/service_user"
	"github.com/SarawinT/user-management-api/middleware"
)

// UserRouter mounts /api/v1/users. Every route needs a valid JWT.
func UserRouter(app *fiber.App, m *middleware.Middleware, db *mongo.Database) {
	userHandler := handler_user.NewUserHandler(
		service_user.NewUserService(mongodb.NewUserRepository(db)),
	)

	userGroup := app.Group("/api/v1/users", m.ValidateJwt())

	userGroup.Get("/", userHandler.ListUser)
	userGroup.Get("/:id", userHandler.InquiryUser)
	userGroup.Patch("/:id", userHandler.UpdateUser)
	userGroup.Delete("/:id", userHandler.DeleteUser)
}
