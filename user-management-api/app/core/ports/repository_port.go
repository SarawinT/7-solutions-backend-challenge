// Package ports defines the interfaces the core services depend on.
// Adapters (repositories, security, ...) implement them.
package ports

import (
	"context"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

// UserPort persists users. Implementations must return
// entity.ErrUserNotFound and entity.ErrEmailAlreadyExists so services stay
// storage-agnostic.
type UserPort interface {
	Create(ctx context.Context, u *entity.UserEntity) error
	GetByID(ctx context.Context, id string) (*entity.UserEntity, error)
	// GetByEmail looks up a user by normalized (lower-case) email.
	GetByEmail(ctx context.Context, email string) (*entity.UserEntity, error)
	List(ctx context.Context, offset, limit int) ([]*entity.UserEntity, int64, error)
	Update(ctx context.Context, u *entity.UserEntity) error
	Delete(ctx context.Context, id string) error
}
