//go:build integration

package sso_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SSpencer740/fastfso/backend/internal/sso"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestStore_Upsert_Create(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")

	cfg, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:      tenantID,
		Protocol:      "saml",
		EntityID:      strPtr("https://idp.acme.com"),
		SSOURL:        strPtr("https://idp.acme.com/sso"),
		Certificate:   strPtr("MIIC..."),
		Enabled:       true,
		AutoProvision: false,
		DefaultRole:   "individual_contributor",
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, cfg.ID)
	assert.Equal(t, tenantID, cfg.TenantID)
	assert.Equal(t, "saml", cfg.Protocol)
	assert.True(t, cfg.Enabled)
}

func TestStore_Upsert_Update(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")

	_, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "saml",
		EntityID:    strPtr("https://idp.acme.com"),
		Enabled:     false,
		DefaultRole: "individual_contributor",
	})
	require.NoError(t, err)

	updated, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		ClientID:    strPtr("client-123"),
		IssuerURL:   strPtr("https://auth.acme.com"),
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)
	assert.Equal(t, "oidc", updated.Protocol)
	assert.True(t, updated.Enabled)
	assert.Equal(t, "fso", updated.DefaultRole)
}

func TestStore_GetByTenantID(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")

	_, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "saml",
		EntityID:    strPtr("https://idp.acme.com"),
		Enabled:     true,
		DefaultRole: "individual_contributor",
	})
	require.NoError(t, err)

	got, err := store.GetByTenantID(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, tenantID, got.TenantID)
	assert.Equal(t, "saml", got.Protocol)
}

func TestStore_GetByTenantID_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	_, err := store.GetByTenantID(ctx, uuid.New())
	assert.ErrorIs(t, err, sso.ErrNotFound)
}

func TestStore_GetByTenantSlug(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")

	_, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "saml",
		EntityID:    strPtr("https://idp.acme.com"),
		Enabled:     true,
		DefaultRole: "individual_contributor",
	})
	require.NoError(t, err)

	cfg, gotTenantID, err := store.GetByTenantSlug(ctx, "acme-corp")
	require.NoError(t, err)
	assert.Equal(t, tenantID, gotTenantID)
	assert.Equal(t, "saml", cfg.Protocol)
}

func TestStore_GetByTenantSlug_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	_, _, err := store.GetByTenantSlug(ctx, "nonexistent")
	assert.ErrorIs(t, err, sso.ErrNotFound)
}

func TestStore_ListEmailDomains(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	cfg, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	_, err = store.AddEmailDomain(ctx, cfg.ID, "acme.com")
	require.NoError(t, err)
	_, err = store.AddEmailDomain(ctx, cfg.ID, "acme.org")
	require.NoError(t, err)

	domains, err := store.ListEmailDomains(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Len(t, domains, 2)
	assert.Equal(t, "acme.com", domains[0].Domain)
	assert.Equal(t, "acme.org", domains[1].Domain)
}

func TestStore_AddEmailDomain_Duplicate(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	cfg, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	_, err = store.AddEmailDomain(ctx, cfg.ID, "acme.com")
	require.NoError(t, err)

	_, err = store.AddEmailDomain(ctx, cfg.ID, "acme.com")
	assert.Error(t, err)
}

func TestStore_RemoveEmailDomain(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	cfg, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	_, err = store.AddEmailDomain(ctx, cfg.ID, "acme.com")
	require.NoError(t, err)

	err = store.RemoveEmailDomain(ctx, cfg.ID, "acme.com")
	require.NoError(t, err)

	domains, err := store.ListEmailDomains(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Empty(t, domains)
}

func TestStore_RemoveEmailDomain_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	cfg, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	err = store.RemoveEmailDomain(ctx, cfg.ID, "nonexistent.com")
	assert.ErrorIs(t, err, sso.ErrNotFound)
}

func TestStore_IsTenantSuspended(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")

	// Not suspended initially
	suspended, err := store.IsTenantSuspended(ctx, tenantID)
	require.NoError(t, err)
	assert.False(t, suspended)

	// Suspend the tenant
	_, err = db.Exec(ctx, "test.suspendTenant", `UPDATE tenants SET suspended_at = now() WHERE id = $1`, tenantID)
	require.NoError(t, err)

	suspended, err = store.IsTenantSuspended(ctx, tenantID)
	require.NoError(t, err)
	assert.True(t, suspended)
}

// TestUpsertParams_JSONUnmarshalsSnakeCase guards against the bug where
// UpsertParams was missing json tags and frontend snake_case fields silently
// mapped to zero values, causing the DB CHECK on default_role to reject the
// upsert with no useful client-facing error.
func TestUpsertParams_JSONUnmarshalsSnakeCase(t *testing.T) {
	body := []byte(`{
		"protocol": "oidc",
		"client_id": "abc.apps.googleusercontent.com",
		"client_secret": "topsecret",
		"issuer_url": "https://accounts.google.com",
		"enabled": true,
		"auto_provision": false,
		"default_role": "individual_contributor"
	}`)
	var p sso.UpsertParams
	require.NoError(t, json.Unmarshal(body, &p))

	require.Equal(t, "oidc", p.Protocol)
	require.NotNil(t, p.ClientID)
	require.Equal(t, "abc.apps.googleusercontent.com", *p.ClientID)
	require.NotNil(t, p.IssuerURL)
	require.Equal(t, "https://accounts.google.com", *p.IssuerURL)
	require.True(t, p.Enabled)
	require.False(t, p.AutoProvision)
	require.Equal(t, "individual_contributor", p.DefaultRole)
}

// TestUpsert_PreservesClientSecretWhenBlank locks in the "leave blank to
// keep existing" UX. Without the COALESCE in the upsert, re-saving with
// client_secret left blank (the default state of the form's masked password
// field) silently nukes the stored credential and locks the tenant out of
// SSO with a "OIDC configuration is incomplete" error on the next sign-in.
func TestUpsert_PreservesClientSecretWhenBlank(t *testing.T) {
	db := testutil.DB(t)
	store := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	original := "topsecret-rotate-me"
	_, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:      tenantID,
		Protocol:      "oidc",
		ClientID:      strPtr("abc.apps.googleusercontent.com"),
		ClientSecret:  strPtr(original),
		IssuerURL:     strPtr("https://accounts.google.com"),
		Enabled:       true,
		AutoProvision: false,
		DefaultRole:   "individual_contributor",
	})
	require.NoError(t, err)

	// Re-save with no client_secret (the most common case from the SSO tab
	// when admins update some other field and leave the masked secret
	// untouched).
	cfg, err := store.Upsert(ctx, sso.UpsertParams{
		TenantID:      tenantID,
		Protocol:      "oidc",
		ClientID:      strPtr("abc.apps.googleusercontent.com"),
		ClientSecret:  nil,
		IssuerURL:     strPtr("https://accounts.google.com"),
		Enabled:       true,
		AutoProvision: true, // changed something else
		DefaultRole:   "individual_contributor",
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.ClientSecret, "client_secret must not be cleared")
	assert.Equal(t, original, *cfg.ClientSecret)

	// Same with explicit empty string — also a "leave alone" signal.
	cfg, err = store.Upsert(ctx, sso.UpsertParams{
		TenantID:      tenantID,
		Protocol:      "oidc",
		ClientID:      strPtr("abc.apps.googleusercontent.com"),
		ClientSecret:  strPtr(""),
		IssuerURL:     strPtr("https://accounts.google.com"),
		Enabled:       true,
		AutoProvision: false,
		DefaultRole:   "individual_contributor",
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.ClientSecret)
	assert.Equal(t, original, *cfg.ClientSecret)

	// But a real new value DOES replace it — we still need rotations to work.
	rotated := "newsecret-after-rotation"
	cfg, err = store.Upsert(ctx, sso.UpsertParams{
		TenantID:      tenantID,
		Protocol:      "oidc",
		ClientID:      strPtr("abc.apps.googleusercontent.com"),
		ClientSecret:  strPtr(rotated),
		IssuerURL:     strPtr("https://accounts.google.com"),
		Enabled:       true,
		AutoProvision: false,
		DefaultRole:   "individual_contributor",
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.ClientSecret)
	assert.Equal(t, rotated, *cfg.ClientSecret)
}
