output "service_url" {
  description = "Cloud Run service URL"
  value       = google_cloud_run_v2_service.backend.uri
}

output "service_name" {
  description = "Cloud Run service name"
  value       = google_cloud_run_v2_service.backend.name
}

output "neg_id" {
  description = "Serverless NEG ID for the load balancer"
  value       = google_compute_region_network_endpoint_group.backend.id
}

output "service_account_email" {
  description = "Backend service account email"
  value       = google_service_account.backend.email
}

output "migrate_job_name" {
  description = "Cloud Run migration job name"
  value       = google_cloud_run_v2_job.migrate.name
}

output "seed_job_name" {
  description = "Cloud Run seed job name"
  value       = google_cloud_run_v2_job.seed.name
}

output "tasks_queue_id" {
  description = "Cloud Tasks email queue ID"
  value       = google_cloud_tasks_queue.email.id
}

output "verify_queue_id" {
  description = "Cloud Tasks AI verification queue ID"
  value       = google_cloud_tasks_queue.verify.id
}
