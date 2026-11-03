package service_auth

import (
	"context"
	"fmt"

	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/utils"
)

func (s *AuthService) Register(ctx context.Context, req request.RegisterRequest) (*entity.UserEntity, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := entity.ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u, err := entity.NewUserEntity(s.ids.NewID(), req.Name, req.Email, hash, utils.NowUTC())
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
