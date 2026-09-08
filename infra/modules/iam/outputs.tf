output "wif_provider_name" {
  description = "Workload Identity Federation provider resource name"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "cicd_service_account_email" {
  description = "CI/CD service account email"
  value       = google_service_account.cicd.email
}
