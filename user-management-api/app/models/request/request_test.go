package request_test

import (
	"errors"
	"testing"

	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
)

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		req     interface{ Validate() error }
		wantMsg string // empty = valid
	}{
		"register ok":           {request.RegisterRequest{Name: "A", Email: "a@x.com", Password: "secret123"}, ""},
		"register all missing":  {request.RegisterRequest{}, "name is required, email is required, password is required"},
		"register blank name":   {request.RegisterRequest{Name: "  ", Email: "a@x.com", Password: "p"}, "name is required"},
		"login ok":              {request.LoginRequest{Email: "a@x.com", Password: "p"}, ""},
		"login missing pw":      {request.LoginRequest{Email: "a@x.com"}, "password is required"},
		"update ok":             {request.UpdateUserRequest{ID: "1", Name: new("B")}, ""},
		"update no fields":      {request.UpdateUserRequest{ID: "1"}, "at least one of name or email is required"},
		"update missing id":     {request.UpdateUserRequest{Name: new("B")}, "id is required"},
		"update empty name set": {request.UpdateUserRequest{ID: "1", Name: new("")}, ""}, // rejected later by the entity
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			var ve *entity.ValidationError
			if !errors.As(err, &ve) || ve.Message != tt.wantMsg {
				t.Fatalf("got %v, want ValidationError %q", err, tt.wantMsg)
			}
		})
	}
}
