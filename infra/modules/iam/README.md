# IAM Module

Configures keyless CI/CD authentication from GitHub Actions via Workload Identity Federation.

## Resources

- Workload Identity Pool for GitHub Actions
- OIDC provider with attribute condition restricting to the fastFSO GitHub org
- CI/CD service account with deployment roles
- WIF binding allowing the repo to impersonate the CI/CD SA

## Security

- No service account keys — uses OIDC tokens from GitHub Actions
- Attribute condition restricts to the specific GitHub organization
- WIF binding restricts to the specific repository
