output "bucket_name" {
  description = "Name of the uploads GCS bucket"
  value       = google_storage_bucket.uploads.name
}
