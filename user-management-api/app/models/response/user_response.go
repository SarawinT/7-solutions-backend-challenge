package response

import (
	"time"

	"github.com/SarawinT/user-management-api/app/models/dto"
	"github.com/SarawinT/user-management-api/app/models/entity"
)

// UserResponse is the public view of a user. PasswordHash is never exposed.
type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ListUserResponse struct {
	Users []UserResponse `json:"users"`
	Total int64          `json:"total"`
}

type LoginResponse struct {
	AccessToken string       `json:"accessToken"`
	TokenType   string       `json:"tokenType"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	User        UserResponse `json:"user"`
}

func NewUserResponse(u *entity.UserEntity) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func NewListUserResponse(d dto.ListUserDto) ListUserResponse {
	users := make([]UserResponse, len(d.Users))
	for i, u := range d.Users {
		users[i] = NewUserResponse(u)
	}
	return ListUserResponse{Users: users, Total: d.Total}
}

func NewLoginResponse(d dto.LoginDto) LoginResponse {
	return LoginResponse{
		AccessToken: d.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   d.ExpiresAt,
		User:        NewUserResponse(d.User),
	}
}
