//go:build integration

package dd254_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastfso/fastfso/backend/internal/dd254"
	"github.com/fastfso/fastfso/backend/internal/testutil"
)

func ptr[T any](v T) *T { return &v }

func createDD254(t *testing.T, store *dd254.Store, tenantID, uploadedBy uuid.UUID, contract, class string, subOrgID *uuid.UUID) *dd254.Form {
	t.Helper()
	id := uuid.New()
	f, err := store.Create(context.Background(), dd254.CreateParams{
		ID:                id,
		TenantID:          tenantID,
		SubOrgID:          subOrgID,
		ContractNumber:    contract,
		ClassificationMax: class,
		StorageKey:        "tenants/x/dd254/" + id.String() + "/file.pdf",
		Filename:          "file.pdf",
		ContentType:       "application/pdf",
		SizeBytes:         123,
		Markings:          dd254.RequiredMarkings,
		UploadedBy:        uploadedBy,
	})
	require.NoError(t, err)
	return f
}

func TestStore_Create_RefusesMarkings(t *testing.T) {
	db := testutil.DB(t)
	store := dd254.NewStore(db)
	tenantID := testutil.CreateTenant(t, db, "Acme")
	identityID := testutil.CreateIdentity(t, db, "u@acme.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	_, err := store.Create(context.Background(), dd254.CreateParams{
		TenantID:          tenantID,
		ContractNumber:    "N00024-23-C-0042",
		ClassificationMax: "secret",
		StorageKey:        "x",
		Filename:          "x.pdf",
		ContentType:       "application/pdf",
		SizeBytes:         1,
		Markings:          "cui",
		UploadedBy:        userID,
	})
	assert.ErrorIs(t, err, dd254.ErrInvalidMarkings)
}

func TestStore_Create_RefusesBadClass(t *testing.T) {
	db := testutil.DB(t)
	store := dd254.NewStore(db)
	tenantID := testutil.CreateTenant(t, db, "Acme")
	identityID := testutil.CreateIdentity(t, db, "u@acme.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	_, err := store.Create(context.Background(), dd254.CreateParams{
		TenantID:          tenantID,
		ContractNumber:    "X",
		ClassificationMax: "purple",
		StorageKey:        "x",
		Filename:          "x.pdf",
		ContentType:       "application/pdf",
		SizeBytes:         1,
		Markings:          dd254.RequiredMarkings,
		UploadedBy:        userID,
	})
	assert.ErrorIs(t, err, dd254.ErrInvalidClass)
}

func TestStore_ListFilterAndStats(t *testing.T) {
	db := testutil.DB(t)
	store := dd254.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subEng := testutil.CreateSubOrg(t, db, tenantID, "Engineering")
	subOps := testutil.CreateSubOrg(t, db, tenantID, "Operations")

	idA := testutil.CreateIdentity(t, db, "admin@acme.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "administrator")

	f1 := createDD254(t, store, tenantID, userA, "N0001", "top_secret", &subEng)
	createDD254(t, store, tenantID, userA, "N0002", "secret", &subOps)
	createDD254(t, store, tenantID, userA, "N0003", "secret", nil) // tenant-wide

	t.Run("FSO scope sees their sub-org + tenant-wide", func(t *testing.T) {
		forms, err := store.List(ctx, dd254.ListFilters{TenantID: tenantID, SubOrgScope: &subEng})
		require.NoError(t, err)
		// f1 (Eng) and the tenant-wide one
		assert.Len(t, forms, 2)
	})

	t.Run("admin sub_org filter is exact", func(t *testing.T) {
		forms, err := store.List(ctx, dd254.ListFilters{TenantID: tenantID, SubOrgID: &subOps})
		require.NoError(t, err)
		require.Len(t, forms, 1)
		assert.Equal(t, "N0002", forms[0].ContractNumber)
	})

	t.Run("class filter", func(t *testing.T) {
		forms, err := store.List(ctx, dd254.ListFilters{TenantID: tenantID, Class: "top_secret"})
		require.NoError(t, err)
		require.Len(t, forms, 1)
		assert.Equal(t, "N0001", forms[0].ContractNumber)
	})

	t.Run("search hits contract", func(t *testing.T) {
		forms, err := store.List(ctx, dd254.ListFilters{TenantID: tenantID, Search: "0001"})
		require.NoError(t, err)
		require.Len(t, forms, 1)
		assert.Equal(t, f1.ID, forms[0].ID)
	})

	t.Run("stats", func(t *testing.T) {
		stats, err := store.Stats(ctx, tenantID, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 3, stats.Active)
		assert.Equal(t, 3, stats.NoReadOn)
	})
}

func TestStore_GrantRevokeAccessAndStats(t *testing.T) {
	db := testutil.DB(t)
	store := dd254.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	idA := testutil.CreateIdentity(t, db, "a@acme.com")
	idB := testutil.CreateIdentity(t, db, "b@acme.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "administrator")
	userB := testutil.CreateUser(t, db, tenantID, idB, "individual_contributor")

	form := createDD254(t, store, tenantID, userA, "X", "secret", nil)

	err := store.GrantAccess(ctx, tenantID, form.ID, userB, userA, ptr(time.Now()), nil)
	require.NoError(t, err)

	detail, err := store.Get(ctx, tenantID, form.ID, nil)
	require.NoError(t, err)
	require.Len(t, detail.AccessGrants, 1)
	assert.Equal(t, userB, detail.AccessGrants[0].UserID)
	assert.Equal(t, 1, detail.ReadOnCount)

	stats, err := store.Stats(ctx, tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.NoReadOn)

	require.NoError(t, store.RevokeAccess(ctx, tenantID, form.ID, userB))
	stats, err = store.Stats(ctx, tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.NoReadOn)
}

func TestStore_SuggestForVisit(t *testing.T) {
	db := testutil.DB(t)
	store := dd254.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	idA := testutil.CreateIdentity(t, db, "a@acme.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "individual_contributor")

	// Two DD254s: one TS that user is read-on, one Secret tenant-wide also read-on.
	tsForm := createDD254(t, store, tenantID, userA, "TS-1", "top_secret", nil)
	sForm := createDD254(t, store, tenantID, userA, "S-1", "secret", nil)

	require.NoError(t, store.GrantAccess(ctx, tenantID, tsForm.ID, userA, userA, nil, nil))
	require.NoError(t, store.GrantAccess(ctx, tenantID, sForm.ID, userA, userA, nil, nil))

	// Create a visit request requiring 'secret'.
	var visitID uuid.UUID
	err := db.QueryRow(ctx, "test.insertVisit",
		`INSERT INTO visit_requests
		 (tenant_id, created_by_user_id, destination_name, diss_smo_code, visit_address,
		  visit_start_date, visit_end_date, access_level, visit_description,
		  poc_name, poc_email, poc_phone, security_poc_name, security_poc_email, security_poc_phone)
		 VALUES ($1, $2, 'Dest', '12345', 'addr', CURRENT_DATE, CURRENT_DATE + 1, 'secret', 'desc',
		         'poc', 'poc@a.com', '555', 'sec', 'sec@a.com', '555')
		 RETURNING id`,
		tenantID, userA,
	).Scan(&visitID)
	require.NoError(t, err)

	suggestions, err := store.SuggestForVisit(ctx, tenantID, visitID)
	require.NoError(t, err)
	// Both DD254s cover 'secret' — the secret one as exact, the TS one as above.
	require.Len(t, suggestions, 2)

	var sawExact, sawAbove bool
	for _, s := range suggestions {
		if s.MatchReason == "exact_class" {
			sawExact = true
			assert.Equal(t, sForm.ID, s.ID)
		}
		if s.MatchReason == "above_class" {
			sawAbove = true
			assert.Equal(t, tsForm.ID, s.ID)
		}
	}
	assert.True(t, sawExact)
	assert.True(t, sawAbove)
}

func TestStore_SetVisitRequestDD254(t *testing.T) {
	db := testutil.DB(t)
	store := dd254.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	idA := testutil.CreateIdentity(t, db, "a@acme.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "administrator")
	form := createDD254(t, store, tenantID, userA, "X", "secret", nil)

	var visitID uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.insertVisit",
		`INSERT INTO visit_requests
		 (tenant_id, created_by_user_id, destination_name, diss_smo_code, visit_address,
		  visit_start_date, visit_end_date, access_level, visit_description,
		  poc_name, poc_email, poc_phone, security_poc_name, security_poc_email, security_poc_phone)
		 VALUES ($1, $2, 'Dest', '12345', 'addr', CURRENT_DATE, CURRENT_DATE + 1, 'secret', 'desc',
		         'poc', 'poc@a.com', '555', 'sec', 'sec@a.com', '555')
		 RETURNING id`,
		tenantID, userA,
	).Scan(&visitID))

	require.NoError(t, store.SetVisitRequestDD254(ctx, tenantID, visitID, &form.ID))

	// Clearing is idempotent.
	require.NoError(t, store.SetVisitRequestDD254(ctx, tenantID, visitID, nil))
}
