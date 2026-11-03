package service_user

import (
	"context"

	"github.com/SarawinT/user-management-api/app/models/dto"
	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/utils"
)

func (s *UserService) ListUser(ctx context.Context, req request.ListUserRequest) (dto.ListUserDto, error) {
	offset, limit := utils.Paginate(req.Page, req.PageSize)
	users, total, err := s.userRepo.List(ctx, offset, limit)
	if err != nil {
		return dto.ListUserDto{}, err
	}
	return dto.ListUserDto{Users: users, Total: total}, nil
}
