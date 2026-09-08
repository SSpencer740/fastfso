# Contributing to fastFSO

Thanks for your interest in contributing!

## Development setup

You'll need Docker and Go installed. The dev tool (`./dev.sh`) manages everything else:

```bash
./dev.sh up      # start postgres + run migrations
./dev.sh seed    # create the local admin account (admin@example.com / admin)
./dev.sh serve   # start backend and frontend dev servers
```

See the [README](README.md) for the full command list.

## Before opening a pull request

- Run `./dev.sh fmt` to format Go, Terraform, and frontend code.
- Run `./dev.sh lint` (golangci-lint + eslint) and fix any findings.
- Run `./dev.sh test` for unit tests, and `./dev.sh test-integration` if you touched any storage-layer code (`Store` types). Storage code requires integration tests (see `CLAUDE.md` for conventions).
- Add unit tests for new code where possible. Unit tests must not depend on a database or external services.

## Conventions

- Backend query methods take a `name` parameter for metrics, in the form `"<package>.<Method>"`.
- If you add or change a `./dev.sh` command, update the dev commands table in `README.md`.
- Run `terraform fmt` after modifying any `.tf` files.

## Reporting bugs and requesting features

Open a GitHub issue. For security vulnerabilities, please follow [SECURITY.md](SECURITY.md) instead of filing a public issue.
