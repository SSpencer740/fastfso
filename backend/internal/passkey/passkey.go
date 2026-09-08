package passkey

import (
	"time"

	"github.com/google/uuid"
)

type Passkey struct {
	ID             uuid.UUID  `json:"id"`
	IdentityID     uuid.UUID  `json:"identity_id"`
	CredentialID   []byte     `json:"-"`
	PublicKey      []byte     `json:"-"`
	AAGUID         []byte     `json:"-"`
	SignCount      int64      `json:"sign_count"`
	BackupEligible bool       `json:"-"`
	BackupState    bool       `json:"-"`
	Transports     []string   `json:"transports"`
	FriendlyName   *string    `json:"friendly_name,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// PublicInfo is the safe-to-return subset of passkey data.
type PublicInfo struct {
	ID           uuid.UUID  `json:"id"`
	FriendlyName *string    `json:"friendly_name,omitempty"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (p *Passkey) Public() PublicInfo {
	return PublicInfo{
		ID:           p.ID,
		FriendlyName: p.FriendlyName,
		LastUsedAt:   p.LastUsedAt,
		CreatedAt:    p.CreatedAt,
	}
}
