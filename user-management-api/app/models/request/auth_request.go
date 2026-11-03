// Package request holds inbound request models shared by the REST and gRPC
// handlers. Validate checks that required fields are present; value rules
// (email format, password length, ...) are enforced by the entity.
package request

import (
	"strings"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r RegisterRequest) Validate() error {
	return requireFields(
		field{"name", r.Name},
		field{"email", r.Email},
		field{"password", r.Password},
	)
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r LoginRequest) Validate() error {
	return requireFields(
		field{"email", r.Email},
		field{"password", r.Password},
	)
}

type field struct {
	name, value string
}

// requireFields reports every missing field at once, e.g.
// "name is required, password is required".
func requireFields(fields ...field) error {
	var missing []string
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name+" is required")
		}
	}
	if len(missing) > 0 {
		return entity.NewValidationError(strings.Join(missing, ", "))
	}
	return nil
}
