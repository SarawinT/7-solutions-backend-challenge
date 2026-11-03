// Package security implements the security ports: JWT tokens (HS256),
// bcrypt password hashing and UUIDv7 ids.
package security

import (
	"errors"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/models/entity"
)

// MinSecretLength is the minimum HS256 key size: a key shorter than the
// 256-bit hash output weakens the signature (RFC 7518 §3.2).
const MinSecretLength = 32

const leeway = 30 * time.Second // tolerated clock skew

var _ ports.TokenPort = (*JwtToken)(nil)

// JwtToken implements ports.TokenPort with JSON Web Tokens signed using
// HMAC-SHA256 (HS256).
type JwtToken struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

type claims struct {
	Email string `json:"email"`
	gojwt.RegisteredClaims
}

// NewJwtToken creates an HS256 token service. secret must be at least
// MinSecretLength bytes.
func NewJwtToken(secret, issuer string, ttl time.Duration) (*JwtToken, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes, got %d", MinSecretLength, len(secret))
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &JwtToken{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}, nil
}

func (s *JwtToken) Issue(userID, email string) (string, *entity.TokenClaimsEntity, error) {
	now := s.now().UTC().Truncate(time.Second) // JWT NumericDate has second precision
	exp := now.Add(s.ttl)

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims{
		Email: email,
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			IssuedAt:  gojwt.NewNumericDate(now),
			NotBefore: gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(exp),
		},
	})
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", nil, fmt.Errorf("sign token: %w", err)
	}
	return signed, &entity.TokenClaimsEntity{UserID: userID, Email: email, IssuedAt: now, ExpiresAt: exp}, nil
}

func (s *JwtToken) Verify(tokenString string) (*entity.TokenClaimsEntity, error) {
	var c claims
	_, err := gojwt.ParseWithClaims(tokenString, &c,
		func(*gojwt.Token) (any, error) { return s.secret, nil },
		// Pinning the algorithm rejects "alg: none" and algorithm-confusion
		// attacks; only HS256 tokens are ever accepted.
		gojwt.WithValidMethods([]string{gojwt.SigningMethodHS256.Alg()}),
		gojwt.WithIssuer(s.issuer),
		gojwt.WithExpirationRequired(),
		gojwt.WithIssuedAt(),
		gojwt.WithLeeway(leeway),
		gojwt.WithTimeFunc(s.now),
	)
	if err != nil || c.Subject == "" {
		return nil, entity.ErrInvalidToken
	}
	return &entity.TokenClaimsEntity{
		UserID:    c.Subject,
		Email:     c.Email,
		IssuedAt:  c.IssuedAt.Time.UTC(),
		ExpiresAt: c.ExpiresAt.Time.UTC(),
	}, nil
}
