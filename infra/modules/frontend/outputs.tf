output "bucket_name" {
  description = "Frontend GCS bucket name"
  value       = google_storage_bucket.frontend.name
}

output "bucket_url" {
  description = "Frontend GCS bucket URL"
  value       = google_storage_bucket.frontend.url
}
