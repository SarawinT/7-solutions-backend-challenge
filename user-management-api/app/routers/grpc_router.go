package routers

import (
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"

	grpcadapter "github.com/SarawinT/user-management-api/app/adapters/grpc"
	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb"
	"github.com/SarawinT/user-management-api/app/adapters/security"
	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/core/services/service_auth"
	"github.com/SarawinT/user-management-api/app/core/services/service_user"
)

// GrpcRouter builds the gRPC server with AuthService (public) and
// UserService (JWT required) on top of the same services as REST.
func GrpcRouter(logger *slog.Logger, db *mongo.Database, token ports.TokenPort, enableReflection bool) *grpc.Server {
	userRepo := mongodb.NewUserRepository(db)
	authService := service_auth.NewAuthService(userRepo, security.NewBcryptHasher(), security.UUIDGenerator{}, token)
	userService := service_user.NewUserService(userRepo)

	return grpcadapter.NewServer(logger, token,
		grpcadapter.NewAuthHandler(authService),
		grpcadapter.NewUserHandler(userService),
		enableReflection,
	)
}
