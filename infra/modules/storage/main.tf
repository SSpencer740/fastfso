# Private GCS bucket for user-uploaded files (task attachments, travel
# documents, wiki PDFs). Files are keyed under per-tenant prefixes by
# backend code and served back to users via short-TTL signed URLs.
resource "google_storage_bucket" "uploads" {
  project                     = var.project_id
  name                        = "fastfso-${var.environment}-uploads"
  location                    = var.location
  force_destroy               = var.force_destroy
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = true
  }
}

# The backend service account reads, writes, and deletes upload objects.
resource "google_storage_bucket_iam_member" "backend_object_admin" {
  bucket = google_storage_bucket.uploads.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${var.backend_service_account_email}"
}

# `SignedURL` on Cloud Run without a static key relies on IAM SignBlob;
# the backend SA must be able to sign as itself.
resource "google_service_account_iam_member" "backend_self_token_creator" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${var.backend_service_account_email}"
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${var.backend_service_account_email}"
}
