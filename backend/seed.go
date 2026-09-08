package main

import (
	"context"
	"fmt"
	"os"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	seedEmail    = "admin@example.com"
	seedPassword = "admin"
	seedName     = "Admin"
)

// The unique index on identities is on lower(email), so ON CONFLICT needs the
// matching expression. The DO UPDATE branch makes seeding idempotent — running
// ./dev.sh seed a second time resets the password/super-admin flag instead of
// failing on the unique constraint.
const seedSQL = `
INSERT INTO identities (email, name, password_hash, is_super_admin, activated)
VALUES ($1, $2, $3, TRUE, TRUE)
ON CONFLICT (lower(email)) DO UPDATE SET
  name = EXCLUDED.name,
  password_hash = EXCLUDED.password_hash,
  is_super_admin = TRUE,
  activated = TRUE;
`

func cmdSeed() {
	ctx := context.Background()

	hash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), 12)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to hash password: %v\n", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, database.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, seedSQL, seedEmail, seedName, string(hash))
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}
}
