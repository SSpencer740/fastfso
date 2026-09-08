package emailcode

import (
	"time"

	"github.com/google/uuid"
)

type Code struct {
	ID         uuid.UUID  `json:"id"`
	IdentityID uuid.UUID  `json:"identity_id"`
	CodeHash   string     `json:"-"`
	Purpose    string     `json:"purpose"`
	ExpiresAt  time.Time  `json:"expires_at"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
