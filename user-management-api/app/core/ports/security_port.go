package ports

import "github.com/SarawinT/user-management-api/app/models/entity"

// TokenPort issues and verifies access tokens.
type TokenPort interface {
	Issue(userID, email string) (token string, claims *entity.TokenClaimsEntity, err error)
	// Verify must return entity.ErrInvalidToken for any token that is
	// malformed, tampered with, expired, or signed with another algorithm.
	Verify(token string) (*entity.TokenClaimsEntity, error)
}

// PasswordHasherPort hashes and verifies plaintext passwords.
type PasswordHasherPort interface {
	Hash(password string) (string, error)
	// Compare returns nil if password matches hash, and an error otherwise.
	Compare(hash, password string) error
}

// IDGeneratorPort produces unique identifiers for new entities.
type IDGeneratorPort interface {
	NewID() string
}
