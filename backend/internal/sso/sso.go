package sso

import (
	"time"

	"github.com/google/uuid"
)

type Config struct {
	ID            uuid.UUID `json:"id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Protocol      string    `json:"protocol"` // "saml" or "oidc"
	EntityID      *string   `json:"entity_id,omitempty"`
	SSOURL        *string   `json:"sso_url,omitempty"`
	Certificate   *string   `json:"certificate,omitempty"`
	ClientID      *string   `json:"client_id,omitempty"`
	ClientSecret  *string   `json:"-"`
	IssuerURL     *string   `json:"issuer_url,omitempty"`
	Enabled       bool      `json:"enabled"`
	AutoProvision bool      `json:"auto_provision"`
	DefaultRole   string    `json:"default_role"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// EmailDomain links an email domain to an SSO configuration.
type EmailDomain struct {
	ID                 uuid.UUID `json:"id"`
	Domain             string    `json:"domain"`
	SSOConfigurationID uuid.UUID `json:"sso_configuration_id"`
	CreatedAt          time.Time `json:"created_at"`
}
