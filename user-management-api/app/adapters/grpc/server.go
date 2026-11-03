package grpc

import (
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/SarawinT/user-management-api/app/core/ports"
	authv1 "github.com/SarawinT/user-management-api/gen/go/auth/v1"
	userv1 "github.com/SarawinT/user-management-api/gen/go/user/v1"
)

// NewServer builds a gRPC server with interceptors, health checks, and
// (optionally) reflection for tools like grpcurl. All RPCs except
// AuthService, health and reflection require a valid JWT.
func NewServer(log *slog.Logger, token ports.TokenPort, authHandler *AuthHandler, userHandler *UserHandler, enableReflection bool) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			LoggingInterceptor(log),  // outermost: logs panics and auth rejections too
			RecoveryInterceptor(log), // turns panics into codes.Internal
			AuthUnaryInterceptor(token),
		),
		grpc.ChainStreamInterceptor(
			AuthStreamInterceptor(token),
		),
	)

	authv1.RegisterAuthServiceServer(srv, authHandler)
	userv1.RegisterUserServiceServer(srv, userHandler)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthSrv.SetServingStatus(authv1.AuthService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthSrv.SetServingStatus(userv1.UserService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, healthSrv)

	if enableReflection {
		reflection.Register(srv)
	}
	return srv
}
