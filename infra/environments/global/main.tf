terraform {
  required_version = ">= 1.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project               = var.project_id
  user_project_override = true
  billing_project       = var.project_id
}

data "google_project" "current" {
  project_id = var.project_id
}

module "services" {
  source = "../../modules/services"

  project_id = var.project_id
}

module "dns_zone" {
  source = "../../modules/dns-zone"

  project_id  = var.project_id
  base_domain = var.base_domain

  depends_on = [module.services]
}

# DNSSEC keys for the cloud.fastfso.com zone — exposes DS records that must be
# published at Cloudflare (the parent DNS for fastfso.com) so that the DNSSEC
# chain of trust extends down into cloud.fastfso.com.
data "google_dns_keys" "cloud" {
  project      = var.project_id
  managed_zone = module.dns_zone.zone_name
}

module "iam" {
  source = "../../modules/iam"

  project_id  = var.project_id
  github_org  = var.github_org
  github_repo = var.github_repo

  depends_on = [module.services]
}

# --- Wildcard SSL certificate via Certificate Manager ---

# DNS authorization for the base domain (validates *.cloud.fastfso.com)
resource "google_certificate_manager_dns_authorization" "wildcard" {
  project = var.project_id
  name    = "fastfso-cloud-wildcard-dns-auth"
  domain  = var.base_domain

  depends_on = [module.services]
}

# CNAME record for DNS authorization validation
resource "google_dns_record_set" "cert_validation" {
  project      = var.project_id
  name         = google_certificate_manager_dns_authorization.wildcard.dns_resource_record[0].name
  type         = google_certificate_manager_dns_authorization.wildcard.dns_resource_record[0].type
  ttl          = 300
  managed_zone = module.dns_zone.zone_name
  rrdatas      = [google_certificate_manager_dns_authorization.wildcard.dns_resource_record[0].data]
}

# Wildcard SSL certificate
resource "google_certificate_manager_certificate" "wildcard" {
  project = var.project_id
  name    = "fastfso-cloud-wildcard-cert"

  managed {
    domains            = ["*.${var.base_domain}"]
    dns_authorizations = [google_certificate_manager_dns_authorization.wildcard.id]
  }
}

# Certificate map (referenced by per-environment load balancers)
resource "google_certificate_manager_certificate_map" "main" {
  project = var.project_id
  name    = "fastfso-cloud-cert-map"

  depends_on = [module.services]
}

# Map entry binding the wildcard cert
resource "google_certificate_manager_certificate_map_entry" "wildcard" {
  project      = var.project_id
  name         = "fastfso-cloud-wildcard-entry"
  map          = google_certificate_manager_certificate_map.main.name
  certificates = [google_certificate_manager_certificate.wildcard.id]
  hostname     = "*.${var.base_domain}"
}

# --- Wildcard SSL certificate for root domain (*.fastfso.com) ---

# DNS authorization for the root domain (validated via Cloudflare DNS)
resource "google_certificate_manager_dns_authorization" "wildcard_root" {
  project = var.project_id
  name    = "fastfso-root-wildcard-dns-auth"
  domain  = var.root_domain

  depends_on = [module.services]
}

# Wildcard SSL certificate for *.fastfso.com
resource "google_certificate_manager_certificate" "wildcard_root" {
  project = var.project_id
  name    = "fastfso-root-wildcard-cert"

  managed {
    domains            = ["*.${var.root_domain}"]
    dns_authorizations = [google_certificate_manager_dns_authorization.wildcard_root.id]
  }
}

# Map entry binding the root wildcard cert
resource "google_certificate_manager_certificate_map_entry" "wildcard_root" {
  project      = var.project_id
  name         = "fastfso-root-wildcard-entry"
  map          = google_certificate_manager_certificate_map.main.name
  certificates = [google_certificate_manager_certificate.wildcard_root.id]
  hostname     = "*.${var.root_domain}"
}

# --- Shared secrets ---

resource "google_secret_manager_secret" "sendgrid_api_key" {
  project   = var.project_id
  secret_id = "fastfso-sendgrid-api-key"

  replication {
    auto {}
  }

  depends_on = [module.services]
}

# --- Artifact Registry (project-wide, shared across environments) ---

resource "google_artifact_registry_repository" "backend" {
  project       = var.project_id
  location      = var.region
  repository_id = "fastfso"
  format        = "DOCKER"
}

# --- Public access exception for frontend buckets ---

resource "google_tags_tag_key" "allow_public_access" {
  parent      = "organizations/${data.google_project.current.org_id}"
  short_name  = "allowPublicAccess"
  description = "Marks resources that may grant allUsers IAM access"
}

resource "google_tags_tag_value" "allow_public_access_true" {
  parent      = google_tags_tag_key.allow_public_access.id
  short_name  = "true"
  description = "Enable public IAM access exception"
}

resource "google_org_policy_policy" "allowed_policy_member_domains" {
  name   = "organizations/${data.google_project.current.org_id}/policies/iam.allowedPolicyMemberDomains"
  parent = "organizations/${data.google_project.current.org_id}"

  spec {
    # Default: restrict to org members only
    rules {
      values {
        allowed_values = ["principalSet://iam.googleapis.com/organizations/${data.google_project.current.org_id}"]
      }
    }

    # Exception: tagged resources allow all principals
    rules {
      allow_all = "TRUE"

      condition {
        expression = "resource.matchTag('${data.google_project.current.org_id}/allowPublicAccess', 'true')"
        title      = "Allow public access on tagged resources"
      }
    }
  }
}
