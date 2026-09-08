# CLAUDE.md

## Project

fastFSO — a tool for Facility Security Officers. The project name is ALWAYS stylized as `fastFSO`.

## Tech Stack

- **Backend:** Go (Gin router, pgx/v5, golang-migrate), linted with golangci-lint
- **Database:** PostgreSQL
- **Frontend:** React + TypeScript, built with Vite, routed with React Router, state via Zustand, linted with ESLint
- **Infrastructure:** Terraform targeting GCP commercial cloud, region `us-east4`. Designed for an FCI-scope boundary (FAR 52.204-21 / CMMC Level 1 controls). It does not run inside Assured Workloads or GovCloud, so the stack as-is is not suitable for CUI, CMMC Level 2, or FedRAMP workloads — do not describe it as such.
- **CI/CD:** GitHub Actions (`.github/workflows/ci.yml`)

## Repo Layout

- `backend/` — Go server, entry point is `main.go`
  - `internal/database/` — pgx connection pool
  - `internal/router/` — Gin route registration
  - `internal/migrate/` — embedded SQL migrations
  - `internal/team/` — roster + clearance history (FSO+ only)
  - `internal/dd254/` — DD254 forms; markings enforced UNCLASSIFIED via DB CHECK (CMMC L1 boundary)
- `frontend/` — React SPA, standard Vite project structure
- `infra/` — Terraform configs
- `tools/dev/` — dev CLI (invoked via `./dev.sh`)

## Dev Tool

When adding or changing `./dev.sh` commands, always update the dev commands table in `README.md` to match.

The primary interface for local development is `./dev.sh`. It manages postgres, migrations, dev servers, tests, and linting.

- `./dev.sh up` — start postgres container and run migrations
- `./dev.sh down` — stop and remove postgres container (keeps data volume)
- `./dev.sh serve` — start backend and frontend dev servers
- `./dev.sh ps` — show postgres container status
- `./dev.sh reset` — remove postgres container and data volume
- `./dev.sh test` — run backend unit tests (`go test ./...`)
- `./dev.sh test-integration` — run backend integration tests against PostgreSQL
- `./dev.sh fmt` — format Go (gofmt + goimports), Terraform, and frontend (eslint --fix)
- `./dev.sh lint` — run golangci-lint (backend) and eslint (frontend)
- `./dev.sh psql` — open a psql shell on the database
- `./dev.sh seed` — create `admin@example.com` super-admin account (password: `admin`)
- `./dev.sh update` — update Go and frontend dependencies
- `./dev.sh registry` — show Artifact Registry image stats (requires `gcloud` auth)
- `./dev.sh registry prune` — delete images older than 30 days (protects `latest`/`staging`/`prod` tags)
- `./dev.sh loc` — lines of code by language (requires `cloc`)
- `./dev.sh clamav up|down|ps` — manage local ClamAV container for malware scan testing (optional; `./dev.sh serve` auto-sets `CLAMAV_ADDR` when the container is running)

### External Database Mode

By default, `./dev.sh` manages a local Docker postgres container. Set environment variables to use an external database instead (e.g., in CI):

- `DB_HOST` — triggers external mode; skips Docker container management
- `DB_PORT` — default `5432` in external mode, `5433` in Docker mode
- `DB_USER` — default `fastfso`
- `DB_PASSWORD` — default `fastfso`
- `DB_NAME` — default `fastfso`
- `DB_SSLMODE` — default `disable`
- `DATABASE_URL` — full connection string (overrides individual vars)
- `DB_MAX_CONNS` — pgxpool size per process; default `5`. Total Postgres connections at peak ≈ `cloud_run_max_instances * DB_MAX_CONNS` — keep that product below the Cloud SQL connection limit.
- `DB_MIN_CONNS` — idle connections kept warm per pool; default `0`. Bump to `1` if cold-start request latency becomes a concern.

### Environment Variables

- `APP_ENV` — application environment: `local` (default), `staging`, `prod`
- `DEPLOY_ENV` — deployment target: `local` (default) or `cloud`
- `FRONTEND_URL` — full frontend origin (default `http://localhost:5173`)
- `BACKEND_URL` — full backend origin (default `http://localhost:8080`)

`IsCloud()` (true when `DEPLOY_ENV=cloud`) controls secure cookies, Cloud Scheduler enforcement, and JSON logging. `IsProduction()` (true when `APP_ENV=prod`) is reserved for prod-only behavior.

## Infrastructure

Always run `terraform fmt` after modifying `.tf` files.

### DNS & Domains

The root domain `fastfso.com` is registered at Cloudflare, with DNS for the apex hosted in Cloudflare DNS (DNS-only / unproxied). GCP manages the `cloud.fastfso.com` subdomain via NS delegation. The canonical user-facing domains are CNAMEs to their `cloud.` counterparts, and the load balancer redirects `*.cloud.fastfso.com` to the canonical domains:

- `app.fastfso.com` → `app.cloud.fastfso.com` — production
- `app-staging.fastfso.com` → `app-staging.cloud.fastfso.com` — staging

A shared wildcard SSL certificate (`*.cloud.fastfso.com`) is created in global infra via Certificate Manager and reused by per-environment load balancers through a certificate map.

## Database Interface

The `database.DB` interface requires a `name` parameter on every query method:

```go
Exec(ctx context.Context, name string, sql string, arguments ...any)
Query(ctx context.Context, name string, sql string, args ...any)
QueryRow(ctx context.Context, name string, sql string, args ...any)
```

The `name` identifies the query for metrics and logging. Convention: `"<package>.<Method>"` — e.g., `"identity.GetByEmail"`, `"session.Create"`. For methods with multiple queries, suffix with a qualifier: `"session.CleanupExpired.revoke"`, `"auth.GetTenantDetail.users"`.

`*pgxpool.Pool` does not satisfy `database.DB` directly — wrap it with `database.PoolAdapter{Pool: pool}`.

## Telemetry

The `internal/telemetry` package instruments the backend with OpenTelemetry metrics exported to GCP Cloud Monitoring (cloud) or discarded (local). When adding new features, consider whether they warrant metrics:

- **Auth flows** (login, registration, 2FA): record success/failure via `telemetry.RecordLogin(ctx, method, success)`
- **Rate limiting**: call `telemetry.RecordRateLimitBlocked(ctx)` when requests are blocked
- **Session creation**: call `telemetry.RecordSessionCreated(ctx)`
- **New query-heavy features**: DB query durations are automatically captured by the `MetricsDB` wrapper — no action needed per query
- **New HTTP routes**: automatically instrumented by the metrics middleware — no action needed per route

If a new category of metric is needed (e.g., a new counter or histogram), add the instrument in `telemetry/metrics.go` with an exported helper function, following the existing pattern.

## Testing

### Unit Tests

Always provide unit tests for new code when possible. Unit tests live alongside the code they test (e.g., `store_test.go` next to `store.go`). Unit tests must not depend on a database or external services — they run via `./dev.sh test` and in CI without any infrastructure.

### Integration Tests

All storage layer code (any `Store` type that wraps `*pgxpool.Pool`) **must** have integration tests. Integration tests are separated from unit tests using the `//go:build integration` build tag.

- **File naming:** `store_integration_test.go` in the same package
- **Database access:** Always use `testutil.DB(t)` — it returns a clean `database.DB` with all tables truncated. Never manage database state manually.
- **Fixtures:** Use `testutil.CreateIdentity(t, pool, email)` and `testutil.CreateTenant(t, pool, name)` for FK setup
- **Assertions:** Use `require` for setup steps (fail fast), `assert` for test assertions
- **Run:** `./dev.sh test-integration` (requires postgres via `./dev.sh up`)
