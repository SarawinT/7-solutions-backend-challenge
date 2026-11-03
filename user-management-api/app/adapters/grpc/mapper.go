package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/SarawinT/user-management-api/app/models/entity"
	userv1 "github.com/SarawinT/user-management-api/gen/go/user/v1"
)

// toProto maps a user entity to its wire representation.
// PasswordHash is deliberately never exposed.
func toProto(u *entity.UserEntity) *userv1.User {
	return &userv1.User{
		Id:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

// toStatus translates entity errors into gRPC status codes.
func toStatus(err error) error {
	if ve, ok := errors.AsType[*entity.ValidationError](err); ok {
		return status.Error(codes.InvalidArgument, ve.Message)
	}

	switch {
	case errors.Is(err, entity.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, entity.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, entity.ErrInvalidName),
		errors.Is(err, entity.ErrInvalidEmail),
		errors.Is(err, entity.ErrInvalidPassword):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, entity.ErrInvalidCredentials),
		errors.Is(err, entity.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		// Don't leak internal details to clients; the logging interceptor
		// records the original error.
		return status.Error(codes.Internal, "internal error")
	}
}
