package security

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/SarawinT/user-management-api/app/core/ports"
)

var _ ports.PasswordHasherPort = BcryptHasher{}

// BcryptHasher implements ports.PasswordHasherPort with bcrypt.
type BcryptHasher struct {
	Cost int
}

func NewBcryptHasher() BcryptHasher {
	return BcryptHasher{Cost: bcrypt.DefaultCost}
}

func (h BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.Cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
