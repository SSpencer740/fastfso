output "lb_static_ip" {
  description = "Load balancer static IP — point DNS here"
  value       = module.load_balancer.static_ip
}

output "cloud_run_url" {
  description = "Cloud Run service URL"
  value       = module.compute.service_url
}

output "frontend_bucket" {
  description = "Frontend GCS bucket name"
  value       = module.frontend.bucket_name
}

output "environment_domain" {
  description = "App domain for this environment"
  value       = local.domain
}

output "db_connection_name" {
  description = "Cloud SQL connection name"
  value       = module.database.connection_name
}

output "migrate_job_name" {
  description = "Cloud Run migration job name"
  value       = module.compute.migrate_job_name
}
