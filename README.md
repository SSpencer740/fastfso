# fastFSO

A tool to help Facility Security Officers (FSOs) do their job — multi-tenant, with per-tenant role-based access.

- **Team roster & clearance tracking** — personnel, clearance levels, and history
- **DD254 management** — upload and track DD Form 254s (markings enforced UNCLASSIFIED)
- **Visit requests** — submit, review, and export visit authorization requests
- **Reporting** — foreign travel reporting with pre/post-travel workflow, plus SEAD-3 style incident/other reporting
- **Tasks & action items** — assignable security tasks with file uploads, AI-assisted document verification, and malware scanning
- **Reminder emails** — scheduled digests and nudges for outstanding items
- **Wiki & AI chat** — per-tenant knowledge base and an AI assistant with tenant context

**Tech stack:** Go (Gin, pgx) + PostgreSQL backend · React + TypeScript (Vite) frontend · Terraform on GCP · GitHub Actions CI/CD.

## Quick Start

You'll need **Docker** and **Go** installed (plus Node for the frontend). The dev tool (`./dev.sh`) manages everything else:

```bash
./dev.sh up      # start postgres container + run migrations
./dev.sh seed    # create the local admin account
./dev.sh serve   # start backend (:8080) and frontend (:5173) dev servers
```

Then open http://localhost:5173 and log in as `admin@example.com` / `admin`.

## Repo Structure

- [`backend/`](backend/) — Go HTTP server
- [`frontend/`](frontend/) — React SPA (Vite + TypeScript)
- [`e2e/`](e2e/) — Playwright end-to-end tests
- [`infra/`](infra/) — Terraform for GCP (architecture, deployment order, and cost notes in [`infra/README.md`](infra/README.md))
- [`features/`](features/) — Feature specs and design docs
- [`CLAUDE.md`](CLAUDE.md) — project conventions for AI coding agents (symlinked as `AGENTS.md`)

## Dev Commands

| Command | Description |
|---|---|
| `./dev.sh up` | Create/start postgres container and run migrations |
| `./dev.sh down` | Stop and remove postgres container (keeps data volume) |
| `./dev.sh serve` | Start backend and frontend dev servers |
| `./dev.sh ps` | Show postgres container status |
| `./dev.sh reset` | Remove postgres container and data volume |
| `./dev.sh test` | Run backend unit tests |
| `./dev.sh test-integration` | Run backend integration tests against PostgreSQL |
| `./dev.sh fmt` | Format Go, Terraform, and frontend code |
| `./dev.sh lint` | Run backend and frontend linters |
| `./dev.sh psql` | Open a psql shell on the database |
| `./dev.sh seed [env]` | Seed admin account locally (default) or execute Cloud Run seed job for a cloud environment |
| `./dev.sh update` | Update Go and frontend dependencies |
| `./dev.sh infra <env> <action>` | Run terraform plan/deploy/destroy for an environment |
| `./dev.sh registry` | Show Artifact Registry image stats |
| `./dev.sh registry prune` | Delete images older than 30 days |
| `./dev.sh loc` | Lines of code by language (requires `cloc`) |
| `./dev.sh e2e` | Run Playwright end-to-end tests |
| `./dev.sh e2e-ui` | Run Playwright tests in interactive UI mode |
| `./dev.sh clamav <up\|down\|ps>` | Manage local ClamAV container for malware scan testing |

## Testing

Unit tests (`./dev.sh test`) run without any infrastructure. Integration tests (`./dev.sh test-integration`) exercise the storage layer against a real PostgreSQL and require `./dev.sh up` first. See [CONTRIBUTING.md](CONTRIBUTING.md) for conventions.

## Deployment

The [`infra/`](infra/) directory contains a complete Terraform setup for running fastFSO on GCP: Cloud Run + Cloud SQL behind a global HTTPS load balancer with Cloud Armor, plus a Workload Identity Federation pool so CI can authenticate keylessly. [`infra/README.md`](infra/README.md) covers the architecture, first-time deployment order, and monthly cost expectations.

Continuous-deployment workflows are not included — [`.github/workflows/ci.yml`](.github/workflows/ci.yml) lints and tests only, and never touches a cloud account. Wiring up a deploy step is left to you.

## Known Limitations

- **SAML SSO is stubbed.** The admin UI lets you select SAML as a protocol, but the backend returns `501 Not Implemented` for both initiation and callback. OIDC is fully implemented.
- **The Playwright e2e suite doesn't run in CI.** The `loginAsAdmin` helper doesn't handle the mandatory TOTP enrollment flow, so the suite times out. It's excluded from the blocking checks in `ci.yml` until the helper is fixed.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). To report a security issue, see [SECURITY.md](SECURITY.md).

## Acknowledgments

Thanks to **Joey Sacchini** for their contributions to fastFSO.

## License

Licensed under the [Apache License 2.0](LICENSE).

The fastFSO name and the logo files in [`Branding/`](Branding/) are trademarks and are **not** covered by the Apache license — please don't use them to represent your own builds or forks of this project.
