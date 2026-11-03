// Package service_auth implements registration, login and token
// verification.
package service_auth

import (
	"context"
	"sync"

	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/models/dto"
	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
)

// AuthServicer is what handlers and auth middleware depend on.
type AuthServicer interface {
	// Register is the only way to create a user.
	Register(ctx context.Context, req request.RegisterRequest) (*entity.UserEntity, error)
	Login(ctx context.Context, req request.LoginRequest) (dto.LoginDto, error)
	// VerifyToken validates an access token and returns its claims, or
	// entity.ErrInvalidToken.
	VerifyToken(ctx context.Context, token string) (*entity.TokenClaimsEntity, error)
}

type AuthService struct {
	userRepo ports.UserPort
	hasher   ports.PasswordHasherPort
	ids      ports.IDGeneratorPort
	tokens   ports.TokenPort

	// dummyHash is compared against when the email is unknown, so a login
	// for a non-existent user costs the same as a wrong password and
	// response timing does not reveal which emails are registered.
	dummyHash func() string
}

func NewAuthService(
	userRepo ports.UserPort,
	hasher ports.PasswordHasherPort,
	ids ports.IDGeneratorPort,
	tokens ports.TokenPort,
) AuthServicer {
	return &AuthService{
		userRepo: userRepo,
		hasher:   hasher,
		ids:      ids,
		tokens:   tokens,
		dummyHash: sync.OnceValue(func() string {
			h, _ := hasher.Hash("dummy-password-for-timing")
			return h
		}),
	}
}
