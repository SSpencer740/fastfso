package sso

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

var ErrNotFound = errors.New("sso configuration not found")

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetByTenantID(ctx context.Context, tenantID uuid.UUID) (*Config, error) {
	c := &Config{}
	err := s.db.QueryRow(ctx, "sso.GetByTenantID",
		`SELECT id, tenant_id, protocol, entity_id, sso_url, certificate,
		        client_id, client_secret, issuer_url, enabled, auto_provision,
		        default_role, created_at, updated_at
		 FROM sso_configurations WHERE tenant_id = $1`,
		tenantID,
	).Scan(
		&c.ID, &c.TenantID, &c.Protocol, &c.EntityID, &c.SSOURL, &c.Certificate,
		&c.ClientID, &c.ClientSecret, &c.IssuerURL, &c.Enabled, &c.AutoProvision,
		&c.DefaultRole, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sso get by tenant: %w", err)
	}
	return c, nil
}

// GetByTenantSlug looks up a tenant by name (used as slug) and returns its SSO config.
func (s *Store) GetByTenantSlug(ctx context.Context, slug string) (*Config, uuid.UUID, error) {
	var tenantID uuid.UUID
	err := s.db.QueryRow(ctx, "sso.GetByTenantSlug",
		`SELECT id FROM tenants WHERE name = $1`, slug,
	).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, uuid.Nil, ErrNotFound
		}
		return nil, uuid.Nil, fmt.Errorf("sso get tenant by slug: %w", err)
	}

	cfg, err := s.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return cfg, tenantID, nil
}

// UpsertParams is bound directly from the JSON body of the SSO config PUT.
// The json tags are load-bearing — without them, encoding/json case-folding
// matches "Protocol" but not "default_role"/"client_id" etc., so the struct
// would arrive with zero values and the DB CHECK constraint on default_role
// would reject the empty string.
type UpsertParams struct {
	TenantID      uuid.UUID `json:"-"`
	Protocol      string    `json:"protocol"`
	EntityID      *string   `json:"entity_id"`
	SSOURL        *string   `json:"sso_url"`
	Certificate   *string   `json:"certificate"`
	ClientID      *string   `json:"client_id"`
	ClientSecret  *string   `json:"client_secret"`
	IssuerURL     *string   `json:"issuer_url"`
	Enabled       bool      `json:"enabled"`
	AutoProvision bool      `json:"auto_provision"`
	DefaultRole   string    `json:"default_role"`
}

func (s *Store) Upsert(ctx context.Context, p UpsertParams) (*Config, error) {
	c := &Config{}
	err := s.db.QueryRow(ctx, "sso.Upsert",
		`INSERT INTO sso_configurations (tenant_id, protocol, entity_id, sso_url, certificate,
		        client_id, client_secret, issuer_url, enabled, auto_provision, default_role)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (tenant_id) DO UPDATE SET
		        protocol = EXCLUDED.protocol, entity_id = EXCLUDED.entity_id,
		        sso_url = EXCLUDED.sso_url, certificate = EXCLUDED.certificate,
		        client_id = EXCLUDED.client_id,
		        -- client_secret is preserved if the request omits it or sends
		        -- empty/whitespace. The "leave blank to keep existing" UX in
		        -- the SSO tab depends on this — without it, a re-save with
		        -- the secret field untouched silently overwrites the stored
		        -- credential with NULL and locks the tenant out of SSO.
		        client_secret = COALESCE(NULLIF(TRIM(EXCLUDED.client_secret), ''), sso_configurations.client_secret),
		        issuer_url = EXCLUDED.issuer_url, enabled = EXCLUDED.enabled,
		        auto_provision = EXCLUDED.auto_provision, default_role = EXCLUDED.default_role,
		        updated_at = now()
		 RETURNING id, tenant_id, protocol, entity_id, sso_url, certificate,
		           client_id, client_secret, issuer_url, enabled, auto_provision,
		           default_role, created_at, updated_at`,
		p.TenantID, p.Protocol, p.EntityID, p.SSOURL, p.Certificate,
		p.ClientID, p.ClientSecret, p.IssuerURL, p.Enabled, p.AutoProvision, p.DefaultRole,
	).Scan(
		&c.ID, &c.TenantID, &c.Protocol, &c.EntityID, &c.SSOURL, &c.Certificate,
		&c.ClientID, &c.ClientSecret, &c.IssuerURL, &c.Enabled, &c.AutoProvision,
		&c.DefaultRole, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sso upsert: %w", err)
	}
	return c, nil
}

func (s *Store) IsTenantSuspended(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	var suspended bool
	err := s.db.QueryRow(ctx, "sso.IsTenantSuspended",
		`SELECT suspended_at IS NOT NULL FROM tenants WHERE id = $1 AND deleted_at IS NULL`, tenantID,
	).Scan(&suspended)
	if err != nil {
		return false, fmt.Errorf("sso store is tenant suspended: %w", err)
	}
	return suspended, nil
}

func (s *Store) ListEmailDomains(ctx context.Context, ssoConfigID uuid.UUID) ([]EmailDomain, error) {
	rows, err := s.db.Query(ctx, "sso.ListEmailDomains",
		`SELECT id, domain, sso_configuration_id, created_at
		 FROM sso_email_domains WHERE sso_configuration_id = $1
		 ORDER BY domain`, ssoConfigID,
	)
	if err != nil {
		return nil, fmt.Errorf("sso store list email domains: %w", err)
	}
	defer rows.Close()

	var domains []EmailDomain
	for rows.Next() {
		var d EmailDomain
		if err := rows.Scan(&d.ID, &d.Domain, &d.SSOConfigurationID, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("sso store list email domains scan: %w", err)
		}
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sso store list email domains rows: %w", err)
	}
	return domains, nil
}

func (s *Store) AddEmailDomain(ctx context.Context, ssoConfigID uuid.UUID, domain string) (EmailDomain, error) {
	var d EmailDomain
	err := s.db.QueryRow(ctx, "sso.AddEmailDomain",
		`INSERT INTO sso_email_domains (sso_configuration_id, domain)
		 VALUES ($1, $2)
		 RETURNING id, domain, sso_configuration_id, created_at`,
		ssoConfigID, domain,
	).Scan(&d.ID, &d.Domain, &d.SSOConfigurationID, &d.CreatedAt)
	if err != nil {
		return d, fmt.Errorf("sso store add email domain: %w", err)
	}
	return d, nil
}

func (s *Store) RemoveEmailDomain(ctx context.Context, ssoConfigID uuid.UUID, domain string) error {
	tag, err := s.db.Exec(ctx, "sso.RemoveEmailDomain",
		`DELETE FROM sso_email_domains
		 WHERE sso_configuration_id = $1 AND domain = $2`,
		ssoConfigID, domain,
	)
	if err != nil {
		return fmt.Errorf("sso store remove email domain: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ProvisionUser(ctx context.Context, identityID, tenantID uuid.UUID, role string) error {
	_, err := s.db.Exec(ctx, "sso.ProvisionUser",
		`INSERT INTO users (identity_id, tenant_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (identity_id, tenant_id) DO NOTHING`,
		identityID, tenantID, role,
	)
	return err
}
