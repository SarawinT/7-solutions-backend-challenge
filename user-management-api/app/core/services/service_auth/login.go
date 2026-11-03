package service_auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/SarawinT/user-management-api/app/models/dto"
	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
)

func (s *AuthService) Login(ctx context.Context, req request.LoginRequest) (dto.LoginDto, error) {
	if err := req.Validate(); err != nil {
		return dto.LoginDto{}, err
	}
	email, err := entity.NormalizeEmail(req.Email)
	if err != nil {
		return dto.LoginDto{}, entity.ErrInvalidCredentials
	}

	u, err := s.userRepo.GetByEmail(ctx, email)
	switch {
	case errors.Is(err, entity.ErrUserNotFound):
		// Spend the same bcrypt time as a wrong password (see dummyHash).
		_ = s.hasher.Compare(s.dummyHash(), req.Password)
		return dto.LoginDto{}, entity.ErrInvalidCredentials
	case err != nil:
		return dto.LoginDto{}, err
	}

	if err := s.hasher.Compare(u.PasswordHash, req.Password); err != nil {
		return dto.LoginDto{}, entity.ErrInvalidCredentials
	}

	token, claims, err := s.tokens.Issue(u.ID, u.Email)
	if err != nil {
		return dto.LoginDto{}, fmt.Errorf("issue token: %w", err)
	}
	return dto.LoginDto{AccessToken: token, ExpiresAt: claims.ExpiresAt, User: u}, nil
}
