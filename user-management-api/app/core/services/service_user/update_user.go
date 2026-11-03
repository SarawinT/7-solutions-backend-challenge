package service_user

import (
	"context"

	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/utils"
)

// UpdateUser applies a partial update: only non-nil fields change.
func (s *UserService) UpdateUser(ctx context.Context, req request.UpdateUserRequest) (*entity.UserEntity, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	u, err := s.userRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	now := utils.NowUTC()
	if req.Name != nil {
		if err := u.Rename(*req.Name, now); err != nil {
			return nil, err
		}
	}
	if req.Email != nil {
		if err := u.ChangeEmail(*req.Email, now); err != nil {
			return nil, err
		}
	}

	if err := s.userRepo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
