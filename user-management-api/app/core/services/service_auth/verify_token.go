package service_auth

import (
	"context"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

func (s *AuthService) VerifyToken(_ context.Context, token string) (*entity.TokenClaimsEntity, error) {
	return s.tokens.Verify(token)
}
