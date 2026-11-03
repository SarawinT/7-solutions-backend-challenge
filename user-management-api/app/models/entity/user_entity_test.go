package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

func TestNew(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name, userName, email string
		wantErr               error
	}{
		{"valid", "Alice", "Alice@Example.com", nil},
		{"empty name", "  ", "a@example.com", entity.ErrInvalidName},
		{"bad email", "Alice", "not-an-email", entity.ErrInvalidEmail},
		{"email with display name", "Alice", "Alice <a@example.com>", entity.ErrInvalidEmail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := entity.NewUserEntity("id", tt.userName, tt.email, "hash", now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
			if err == nil && u.Email != "alice@example.com" {
				t.Fatalf("email not normalized: %q", u.Email)
			}
		})
	}
}
