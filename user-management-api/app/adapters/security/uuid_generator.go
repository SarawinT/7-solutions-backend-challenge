package security

import (
	"github.com/google/uuid"

	"github.com/SarawinT/user-management-api/app/core/ports"
)

var _ ports.IDGeneratorPort = UUIDGenerator{}

// UUIDGenerator implements ports.IDGeneratorPort with time-ordered UUIDv7.
type UUIDGenerator struct{}

func (UUIDGenerator) NewID() string { return uuid.Must(uuid.NewV7()).String() }
