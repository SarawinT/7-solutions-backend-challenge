package service_user

import (
	"context"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	if id == "" {
		return entity.NewValidationError("id is required")
	}
	return s.userRepo.Delete(ctx, id)
}
