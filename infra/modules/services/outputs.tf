output "enabled_apis" {
  description = "Set of enabled API service names"
  value       = { for k, v in google_project_service.api : k => v.service }
}
