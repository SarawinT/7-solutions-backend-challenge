// Package entity holds the core business entities and their rules. It
// depends only on the Go standard library.
package entity

import (
	"net/mail"
	"strings"
	"time"
)

const (
	maxNameLength     = 100
	minPasswordLength = 8
)

// UserEntity is a registered user.
type UserEntity struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUserEntity creates a validated user. The password must already be
// hashed (see ports.PasswordHasherPort).
func NewUserEntity(id, name, email, passwordHash string, now time.Time) (*UserEntity, error) {
	name, err := normalizeName(name)
	if err != nil {
		return nil, err
	}
	email, err = NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	return &UserEntity{
		ID:           id,
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Rename changes the user's display name.
func (u *UserEntity) Rename(name string, now time.Time) error {
	name, err := normalizeName(name)
	if err != nil {
		return err
	}
	u.Name = name
	u.UpdatedAt = now
	return nil
}

// ChangeEmail changes the user's email address.
func (u *UserEntity) ChangeEmail(email string, now time.Time) error {
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	u.Email = email
	u.UpdatedAt = now
	return nil
}

// ValidatePassword checks a plaintext password against the password rules.
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrInvalidPassword
	}
	return nil
}

// NormalizeEmail validates an email and returns its canonical form
// (trimmed, lower-case), as stored on UserEntity.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameLength {
		return "", ErrInvalidName
	}
	return name, nil
}
