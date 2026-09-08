# Secrets

All secrets are stored in [Google Secret Manager](https://console.cloud.google.com/security/secret-manager?project=fastfso).

## Inventory

| Secret | Secret Manager ID | Scope | Provisioned by |
|---|---|---|---|
| Database password | `fastfso-{environment}-db-password` | Per-environment | Terraform (auto-generated) |
| SendGrid API key | `fastfso-sendgrid-api-key` | Global (shared) | Terraform (shell created), value loaded manually |

### Database password

Created automatically by the `database` module during `terraform apply`. A random 32-character password is generated and stored in Secret Manager. No manual action is needed.

Each environment has its own secret (e.g., `fastfso-staging-db-password`, `fastfso-prod-db-password`).

### SendGrid API key

The secret shell is created by the `global` environment Terraform config. The actual API key value must be loaded manually after the first `terraform apply`:

```sh
echo -n "SG.your-api-key-here" | \
  gcloud secrets versions add fastfso-sendgrid-api-key \
    --project=fastfso \
    --data-file=-
```

To verify the value was stored:

```sh
gcloud secrets versions access latest \
  --secret=fastfso-sendgrid-api-key \
  --project=fastfso
```
