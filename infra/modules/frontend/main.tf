resource "google_storage_bucket" "frontend" {
  project                     = var.project_id
  name                        = "fastfso-${var.environment}-frontend"
  location                    = var.location
  force_destroy               = var.force_destroy
  uniform_bucket_level_access = true

  website {
    main_page_suffix = "index.html"
    not_found_page   = "index.html"
  }

  cors {
    origin          = ["https://${var.domain}"]
    method          = ["GET", "HEAD"]
    response_header = ["Content-Type"]
    max_age_seconds = 3600
  }
}

resource "google_tags_location_tag_binding" "public_access" {
  parent    = "//storage.googleapis.com/projects/_/buckets/${google_storage_bucket.frontend.name}"
  tag_value = var.public_access_tag_value_id
  location  = lower(google_storage_bucket.frontend.location)
}

resource "google_storage_bucket_iam_member" "public_read" {
  bucket = google_storage_bucket.frontend.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"

  depends_on = [google_tags_location_tag_binding.public_access]
}

