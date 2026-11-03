package entity

import "time"

// TokenClaimsEntity is the authenticated identity carried by an access token.
type TokenClaimsEntity struct {
	UserID    string
	Email     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}
