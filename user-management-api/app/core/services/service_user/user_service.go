// Package service_user implements the user management use cases.
package service_user

import (
	"context"

	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/models/dto"
	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
)

// UserServicer is what handlers depend on. Users are created through
// service_auth.AuthServicer.Register.
type UserServicer interface {
	InquiryUser(ctx context.Context, id string) (*entity.UserEntity, error)
	ListUser(ctx context.Context, req request.ListUserRequest) (dto.ListUserDto, error)
	UpdateUser(ctx context.Context, req request.UpdateUserRequest) (*entity.UserEntity, error)
	DeleteUser(ctx context.Context, id string) error
}

type UserService struct {
	userRepo ports.UserPort
}

func NewUserService(userRepo ports.UserPort) UserServicer {
	return &UserService{userRepo: userRepo}
}
