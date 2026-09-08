package totp

import (
	"time"

	"github.com/google/uuid"
)

type Secret struct {
	ID         uuid.UUID `json:"id"`
	IdentityID uuid.UUID `json:"identity_id"`
	Secret     string    `json:"-"`
	Verified   bool      `json:"verified"`
	CreatedAt  time.Time `json:"created_at"`
}
