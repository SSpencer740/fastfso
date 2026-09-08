# fastFSO Infrastructure

Terraform infrastructure for fastFSO on GCP.

## Architecture

```
Internet → Cloud DNS (cloud.fastfso.com zone) → Static IP → Cloud Armor WAF
  → HTTPS LB (TLS 1.2+ RESTRICTED, wildcard cert *.cloud.fastfso.com)
    /api/*  → Cloud Run v2 (Go backend, Direct VPC Egress)
    /*      → GCS Bucket (React SPA, CDN enabled)

Cloud Run → Private Subnet (10.0.0.0/20) → Cloud SQL PostgreSQL 16
                                            (private IP only, SSL required)

GitHub Actions → Workload Identity Federation → GCP (keyless)
```

## Domain Structure

Root domain (`fastfso.com`) is registered at Cloudflare with DNS hosted in Cloudflare DNS (DNS-only / unproxied). GCP manages the `cloud.fastfso.com` subdomain via NS delegation from Cloudflare.

| Domain | Purpose |
|---|---|
| `fastfso.com` | Landing page (apex A record) |
| `cloud.fastfso.com` | GCP DNS zone (NS delegated from Cloudflare) |
| `app.cloud.fastfso.com` | Prod app (React SPA + Go API) |
| `app-staging.cloud.fastfso.com` | Staging app |

## Directory Structure

```
infra/
  environments/
    global/           ← shared infra (DNS zone, wildcard cert, IAM)
    staging/          ← staging environment (auto-deploys on merge to main)
    prod/             ← production environment (manual deploy via GitHub Actions)
  modules/
    services/         ← GCP API enablement
    networking/       ← VPC, subnet, firewalls
    database/         ← Cloud SQL PostgreSQL, Secret Manager
    compute/          ← Cloud Run v2, Artifact Registry, serverless NEG
    frontend/         ← GCS bucket for SPA hosting
    load-balancer/    ← HTTPS LB, Cloud Armor, SSL
    iam/              ← Workload Identity Federation for CI/CD
    dns-zone/         ← Cloud DNS zone (global)
    dns/              ← Per-environment DNS records
```

## Deployment Order

1. Apply `environments/global` first (DNS zone + wildcard cert)
2. Configure NS records at Cloudflare for `cloud.fastfso.com` → GCP nameservers
3. Add your SendGrid API key as a secret version to `fastfso-sendgrid-api-key` in Secret Manager
4. Apply per-environment configs in order: staging → prod

## Usage

### Validate

```bash
# Global environment
cd infra/environments/global
terraform init -backend=false
terraform validate

# Staging environment
cd infra/environments/staging
terraform init -backend=false
terraform validate

# Format check
terraform fmt -check -recursive infra/
```

### Deploy Global (first time)

```bash
cd infra/environments/global
terraform init
terraform plan
terraform apply
```

### Deploy an Environment

```bash
cd infra/environments/staging   # or prod
terraform init
terraform plan
terraform apply
```

### After First Apply

1. Add NS records at Cloudflare for `cloud.fastfso.com` pointing to the `name_servers` output from global
2. Wait for DNS propagation — the wildcard SSL cert will auto-provision via Certificate Manager
3. Replace the placeholder `container_image` with a real backend image in `terraform.tfvars`

### Deploying the App

This repo does **not** ship continuous-deployment workflows — only CI
(`.github/workflows/ci.yml`, which lints and tests but never touches a cloud
account). Bring your own deploy step.

The `iam/` module provisions a Workload Identity Federation pool and a CI
service account, so a GitHub Actions job can authenticate to GCP keylessly
(`google-github-actions/auth` with `workload_identity_provider`). From there a
deploy is: build and push the backend image to Artifact Registry, update the
Cloud Run service, run the `migrate` job, and sync the built SPA to the frontend
bucket.

## Estimated Monthly Cost

Almost every cost here is **fixed** — it accrues at zero traffic. Treat idle
spend as the baseline, not as something usage will amortize.

Prod, with staging torn down (see below), is roughly **$145/month**:

| Component | Approx. cost/month | Notes |
|---|---|---|
| Cloud SQL (`db-custom-1-3840`, zonal) | ~$53 | 1 vCPU @ ~$0.0442/h + 3.75 GiB @ ~$0.0075/GiB·h |
| Cloud Run (`min_instance_count = 1`) | ~$29 | 2 vCPU + 2.5 GiB held warm at the idle CPU rate |
| Load balancer | ~$36 | ~$18 per forwarding rule; two of them (HTTPS + HTTP→HTTPS redirect) |
| Cloud Armor | ~$16 | $5/policy + ~$1/rule × 11 rules |
| Storage, Artifact Registry, secrets | ~$10 | |

Standing staging back up adds roughly **$80/month** on top (its own DB, load
balancer, and Armor policy).

### Why prod's Cloud Run bill is what it is

Cloud Run scales to zero in **staging only**. Prod sets `min_instance_count = 1`
(`environments/prod/variables.tf`) to avoid cold starts, and that warm instance
holds 2 vCPU / 2.5 GiB — the backend is 1 vCPU / 512 MiB and the ClamAV sidecar
adds another 1 vCPU / 2 GiB.

Both containers set `cpu_idle = true`, which matters more than it looks. The
provider defaults it to **false** ("CPU always allocated"), which bills the warm
instance at the always-on rate of $0.000018/vCPU-s around the clock (~$106/month
for this shape). The idle rate is $0.0000025/vCPU-s, 7.2× cheaper. If you ever
see Cloud Run spend jump without a traffic change, check this flag first.

The remaining ~$29/month is the price of no cold starts. Getting to ~$0 means
`min_instance_count = 0`, which ClamAV currently blocks: clamd's startup probe
gates instance readiness for 30-60s while it loads signatures, so the first
request after a scale-down pays that. Decoupling it means fronting clamd with an
HTTP shim, since Cloud Run only accepts HTTP and clamd's INSTREAM protocol is
raw TCP. That trade — days of work and a slower upload path for ~$29/month — has
not been worth making so far.

### Checking real numbers

There is no BigQuery billing export, so actual spend has to be read from the
console (Billing → Reports). Usage is measurable from the CLI, though:

```bash
# Instance-seconds actually billed (prod, last 30d)
curl -s -H "Authorization: Bearer $(gcloud auth print-access-token)" \
  --get "https://monitoring.googleapis.com/v3/projects/fastfso/timeSeries" \
  --data-urlencode 'filter=metric.type="run.googleapis.com/container/billable_instance_time" AND resource.labels.service_name="fastfso-prod-backend"' \
  --data-urlencode "interval.startTime=$(date -u -v-30d +%Y-%m-%dT%H:%M:%SZ)" \
  --data-urlencode "interval.endTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --data-urlencode 'aggregation.alignmentPeriod=2592000s' \
  --data-urlencode 'aggregation.perSeriesAligner=ALIGN_SUM'
```

Unit prices come from the Cloud Billing Catalog API (Cloud Run is service
`152E-C115-5142`, Cloud SQL is `9662-B51E-5089`) — cheaper than trusting a
remembered price.

## Tearing Down Staging

Staging is disposable and costs ~$70/month to leave running, so destroy it when
it is not in use. A re-apply recreates an exact mirror of prod's topology, which
is the point — degrading staging to save money (dropping its load balancer, say)
would cost single-origin serving and make it useless for validating prod's
cookie and CORS behavior.

```bash
cd infra/environments/staging
terraform apply     # required once: writes deletion_protection=false to state
terraform destroy
```

The `apply` is not optional the first time. `deletion_protection` is a
provider-side guard that Terraform reads from **state**, not from config, so a
`destroy` run before the flag has been applied fails with the protection still
in force. The apply makes no change in GCP — the field never leaves Terraform.

If you wire up a CD workflow that applies staging on every merge, gate it behind
a repo variable so a teardown does not silently reverse itself on the next push.

Bringing it back takes ~10-15 minutes:

```bash
cd infra/environments/staging
terraform apply
gcloud run jobs execute fastfso-staging-migrate --region us-east4 --wait
gcloud run jobs execute fastfso-staging-seed --region us-east4 --wait
```

Notes:

- **Staging data does not survive.** The SQL instance and its backups are
  deleted; `migrate` + `seed` rebuild from scratch. Never point this at prod —
  prod keeps `deletion_protection = true`.
- The load balancer gets a **new static IP** on re-apply. Terraform updates the
  `app-staging.cloud.fastfso.com` record itself, but allow for DNS propagation
  before the domain resolves.

### Teardown blockers

A destroy touches ~70 resources and several guards fire late, after most of the
environment is already gone. Staging now sets the flags that clear the ones that
can be fixed in config — `force_destroy` on both buckets, `deletion_protection`
on the Cloud Run service and jobs, `deletion_policy = "ABANDON"` on the SQL user.
Prod keeps every default. Two blockers remain that config cannot solve:

- **Org-level tag binding (403).** `google_tags_location_tag_binding` on the
  frontend bucket is parented to `organizations/…`, so project `owner` is not
  enough — only the CI service account has the org-level tag permission. Running
  the destroy from CI avoids this. From a laptop, drop it from state and let the
  bucket deletion reap it (a tag binding cannot outlive its resource):
  ```bash
  terraform state rm module.frontend.google_tags_location_tag_binding.public_access
  ```
  Note this makes a subsequent `terraform apply` try to re-create it and fail the
  same way — harmless mid-teardown, but do not leave staging in that state.
- **Serverless VPC address lag.** Direct VPC egress leaves a
  `serverless-ipv4-*` address holding the subnet, and it is reserved by
  `serverless.googleapis.com` for a while after Cloud Run is deleted — so the
  subnet and VPC deletion fail on `resourceInUseByAnotherResource`. These are
  free, so it is fine to stop there and leave the VPC in place; it makes the
  restore faster. **Check the subnet before deleting any such address — prod has
  one too:**
  ```bash
  gcloud compute addresses describe NAME --region us-east4 --format='value(subnetwork)'
  ```

Stopping with the VPC, subnet, peering, and API enablements still present is the
expected end state. All are free, and `terraform apply` rebuilds the rest on top
of them.
