output "dns_zone_name" {
  description = "Cloud DNS managed zone name — used by per-environment configs"
  value       = module.dns_zone.zone_name
}

output "name_servers" {
  description = "Name servers — configure as NS records at Cloudflare for cloud.fastfso.com"
  value       = module.dns_zone.name_servers
}

output "cloud_dns_ds_records" {
  description = "DS records to publish at Cloudflare (on the cloud subdomain) to extend the DNSSEC chain of trust into cloud.fastfso.com"
  value       = data.google_dns_keys.cloud.key_signing_keys[*].ds_record
}

output "certificate_map_id" {
  description = "Certificate Manager certificate map ID for shared wildcard cert"
  value       = "//certificatemanager.googleapis.com/${google_certificate_manager_certificate_map.main.id}"
}

output "root_wildcard_dns_auth_record" {
  description = "CNAME record to add at Cloudflare for *.fastfso.com certificate validation"
  value = {
    name = google_certificate_manager_dns_authorization.wildcard_root.dns_resource_record[0].name
    type = google_certificate_manager_dns_authorization.wildcard_root.dns_resource_record[0].type
    data = google_certificate_manager_dns_authorization.wildcard_root.dns_resource_record[0].data
  }
}

output "wif_provider" {
  description = "Workload Identity Federation provider for GitHub Actions"
  value       = module.iam.wif_provider_name
}

output "cicd_service_account" {
  description = "CI/CD service account email"
  value       = module.iam.cicd_service_account_email
}

output "artifact_registry_repo" {
  description = "Artifact Registry repository name"
  value       = google_artifact_registry_repository.backend.name
}

output "sendgrid_api_key_secret_id" {
  description = "Secret Manager secret ID for the SendGrid API key"
  value       = google_secret_manager_secret.sendgrid_api_key.secret_id
}

output "public_access_tag_value_id" {
  description = "Tag value ID to bind to resources that need allUsers IAM access"
  value       = google_tags_tag_value.allow_public_access_true.id
}
