//go:build integration

package testutil

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fastfso/fastfso/backend/internal/database"
	"github.com/fastfso/fastfso/backend/internal/env"
	"github.com/fastfso/fastfso/backend/internal/migrate"
)

var (
	pool     *pgxpool.Pool
	initOnce sync.Once
	initErr  error
)

const truncateSQL = `
TRUNCATE TABLE
    chat_messages,
    reports,
    travel_report_uploads, travel_report_countries, travel_reports,
    visit_requests,
    task_uploads, task_responses, task_completions, task_assignment_rules,
    task_requirements, action_items, tasks,
    auth_audit_log, sessions, sso_email_domains, sso_configurations,
    email_codes, totp_secrets, passkeys,
    dd254_user_access, dd254_forms,
    user_clearance_records, user_suborganizations, users,
    suborganizations, tenants, identities
CASCADE
`

// DB returns a shared connection pool to the integration test database.
// On first call it creates the test database (if needed), runs migrations,
// and establishes a connection pool. Every call truncates all tables so each
// test starts with a clean slate.
func DB(t *testing.T) database.DB {
	t.Helper()

	initOnce.Do(func() {
		dsn := env.IntegrationDatabaseURL()
		if dsn == "" {
			initErr = fmt.Errorf("INTEGRATION_DATABASE_URL is not set")
			return
		}

		ctx := context.Background()

		// Extract the test database name from the DSN.
		u, err := url.Parse(dsn)
		if err != nil {
			initErr = fmt.Errorf("parse DSN: %w", err)
			return
		}
		dbName := strings.TrimPrefix(u.Path, "/")

		// Connect to the default "postgres" database to create the test DB.
		adminDSN := replaceDBName(u, "postgres")
		adminPool, err := pgxpool.New(ctx, adminDSN)
		if err != nil {
			initErr = fmt.Errorf("connect to postgres database: %w", err)
			return
		}

		_, err = adminPool.Exec(ctx,
			fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{dbName}.Sanitize()))
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			adminPool.Close()
			initErr = fmt.Errorf("create test database: %w", err)
			return
		}
		adminPool.Close()

		// Run migrations on the test database.
		if err := migrate.Up(dsn); err != nil {
			initErr = fmt.Errorf("migrate test database: %w", err)
			return
		}

		// Connect to the test database.
		pool, err = pgxpool.New(ctx, dsn)
		if err != nil {
			initErr = fmt.Errorf("connect to test database: %w", err)
			return
		}
	})

	if initErr != nil {
		t.Fatal(initErr)
	}

	// Truncate all tables before each test.
	ctx := context.Background()
	if _, err := pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}

	return &database.PoolAdapter{Pool: pool}
}

// CreateIdentity inserts a minimal identity row and returns its UUID.
// The display name is derived from the email local part with the first letter capitalized
// (e.g. "alice@example.com" → "Alice"). The identity is created in an activated
// state — tests that need a deactivated identity should update the row directly.
func CreateIdentity(t *testing.T, db database.DB, email string) uuid.UUID {
	t.Helper()
	name := email
	if i := strings.Index(email, "@"); i > 0 {
		name = strings.ToUpper(email[:1]) + email[1:i]
	}
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "testutil.CreateIdentity",
		`INSERT INTO identities (email, name, activated) VALUES ($1, $2, TRUE) RETURNING id`,
		email, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create identity %q: %v", email, err)
	}
	return id
}

// CreateTenant inserts a minimal tenant row and returns its UUID.
func CreateTenant(t *testing.T, db database.DB, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "testutil.CreateTenant",
		`INSERT INTO tenants (name) VALUES ($1) RETURNING id`,
		name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create tenant %q: %v", name, err)
	}
	return id
}

// CreateUser inserts a user row for the given tenant and identity with the specified role.
func CreateUser(t *testing.T, db database.DB, tenantID, identityID uuid.UUID, role string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "testutil.CreateUser",
		`INSERT INTO users (tenant_id, identity_id, role) VALUES ($1, $2, $3) RETURNING id`,
		tenantID, identityID, role,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

// CreateSubOrg inserts a suborganization for the given tenant.
func CreateSubOrg(t *testing.T, db database.DB, tenantID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "testutil.CreateSubOrg",
		`INSERT INTO suborganizations (tenant_id, name) VALUES ($1, $2) RETURNING id`,
		tenantID, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create sub org %q: %v", name, err)
	}
	return id
}

// AssignUserToSubOrg links a user to a suborganization.
func AssignUserToSubOrg(t *testing.T, db database.DB, userID, subOrgID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(context.Background(), "testutil.AssignUserToSubOrg",
		`INSERT INTO user_suborganizations (user_id, suborganization_id) VALUES ($1, $2)`,
		userID, subOrgID,
	)
	if err != nil {
		t.Fatalf("assign user to sub org: %v", err)
	}
}

func replaceDBName(u *url.URL, name string) string {
	cp := *u
	cp.Path = "/" + name
	return cp.String()
}
