// Package grpc is the inbound gRPC handler. It translates protobuf
// messages into request models and entity errors into gRPC statuses.
// It depends only on the service interfaces, never on concrete services.
package grpc

import (
	"context"

	"github.com/SarawinT/user-management-api/app/core/services/service_user"
	"github.com/SarawinT/user-management-api/app/models/request"
	userv1 "github.com/SarawinT/user-management-api/gen/go/user/v1"
)

type UserHandler struct {
	userv1.UnimplementedUserServiceServer
	svc service_user.UserServicer
}

func NewUserHandler(svc service_user.UserServicer) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	u, err := h.svc.InquiryUser(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &userv1.GetUserResponse{User: toProto(u)}, nil
}

func (h *UserHandler) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	res, err := h.svc.ListUser(ctx, request.ListUserRequest{
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	users := make([]*userv1.User, len(res.Users))
	for i, u := range res.Users {
		users[i] = toProto(u)
	}
	return &userv1.ListUsersResponse{Users: users, Total: res.Total}, nil
}

func (h *UserHandler) UpdateUser(ctx context.Context, req *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	u, err := h.svc.UpdateUser(ctx, request.UpdateUserRequest{
		ID:    req.GetId(),
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &userv1.UpdateUserResponse{User: toProto(u)}, nil
}

func (h *UserHandler) DeleteUser(ctx context.Context, req *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
	if err := h.svc.DeleteUser(ctx, req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &userv1.DeleteUserResponse{}, nil
}
