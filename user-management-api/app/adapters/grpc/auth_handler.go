package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/SarawinT/user-management-api/app/core/services/service_auth"
	"github.com/SarawinT/user-management-api/app/models/request"
	authv1 "github.com/SarawinT/user-management-api/gen/go/auth/v1"
)

type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	auth service_auth.AuthServicer
}

func NewAuthHandler(auth service_auth.AuthServicer) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	u, err := h.auth.Register(ctx, request.RegisterRequest{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &authv1.RegisterResponse{User: toProto(u)}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	res, err := h.auth.Login(ctx, request.LoginRequest{Email: req.GetEmail(), Password: req.GetPassword()})
	if err != nil {
		return nil, toStatus(err)
	}
	return &authv1.LoginResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   timestamppb.New(res.ExpiresAt),
		User:        toProto(res.User),
	}, nil
}
