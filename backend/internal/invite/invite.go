package invite

import (
	"time"

	"github.com/google/uuid"
)

// Token represents an invite token stored in the email_codes table.
type Token struct {
	ID         uuid.UUID
	IdentityID uuid.UUID
	ExpiresAt  time.Time
	UsedAt     *time.Time
}
