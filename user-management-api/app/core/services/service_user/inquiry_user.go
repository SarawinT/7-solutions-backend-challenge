package service_user

import (
	"context"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

func (s *UserService) InquiryUser(ctx context.Context, id string) (*entity.UserEntity, error) {
	if id == "" {
		return nil, entity.NewValidationError("id is required")
	}
	return s.userRepo.GetByID(ctx, id)
}
