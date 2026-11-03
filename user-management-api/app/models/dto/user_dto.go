// Package dto holds the results services return to handlers.
package dto

import (
	"time"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

type ListUserDto struct {
	Users []*entity.UserEntity
	Total int64
}

type LoginDto struct {
	AccessToken string
	ExpiresAt   time.Time
	User        *entity.UserEntity
}
