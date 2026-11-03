package request

import "github.com/SarawinT/user-management-api/app/models/entity"

type ListUserRequest struct {
	Page     int `query:"page"`     // zero-based
	PageSize int `query:"pageSize"` // default 20, max 100
}

type UpdateUserRequest struct {
	ID    string  `json:"-"`     // from the URL path
	Name  *string `json:"name"`  // nil = unchanged
	Email *string `json:"email"` // nil = unchanged
}

func (r UpdateUserRequest) Validate() error {
	if r.ID == "" {
		return entity.NewValidationError("id is required")
	}
	if r.Name == nil && r.Email == nil {
		return entity.NewValidationError("at least one of name or email is required")
	}
	return nil
}
