# Dedicated service account for the backend
resource "google_service_account" "backend" {
  project      = var.project_id
  account_id   = "fastfso-${var.environment}-backend"
  display_name = "fastFSO ${var.environment} backend"
}

resource "google_project_iam_member" "backend_cloudsql" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_project_iam_member" "backend_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_project_iam_member" "backend_monitoring" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_project_iam_member" "backend_tasks_enqueuer" {
  project = var.project_id
  role    = "roles/cloudtasks.enqueuer"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_project_iam_member" "backend_vertex_ai" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

data "google_project" "current" {
  project_id = var.project_id
}

resource "google_service_account_iam_member" "cloudtasks_token_creator" {
  service_account_id = google_service_account.backend.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-cloudtasks.iam.gserviceaccount.com"
}

# Backend SA needs actAs on itself to specify itself as the OIDC token identity when creating tasks
resource "google_service_account_iam_member" "backend_self_actas" {
  service_account_id = google_service_account.backend.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.backend.email}"
}

# Grant secret access on the specific DB password secret only
resource "google_secret_manager_secret_iam_member" "backend_secret_access" {
  project   = var.project_id
  secret_id = var.db_password_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend.email}"
}

# Grant secret access on the SendGrid API key secret
resource "google_secret_manager_secret_iam_member" "backend_sendgrid_secret_access" {
  project   = var.project_id
  secret_id = var.sendgrid_api_key_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend.email}"
}

# Cloud Tasks queue for async email delivery
resource "google_cloud_tasks_queue" "email" {
  project  = var.project_id
  location = var.region
  name     = "fastfso-${var.environment}-email"

  rate_limits {
    max_dispatches_per_second = 100
    max_concurrent_dispatches = 10
  }

  stackdriver_logging_config {
    sampling_ratio = var.task_log_sampling_ratio
  }

  retry_config {
    max_attempts       = 5
    max_retry_duration = "3600s"
    min_backoff        = "1s"
    max_backoff        = "60s"
    max_doublings      = 4
  }
}

# Cloud Tasks queue for AI upload verification jobs. Lower rate limits than
# email because each task triggers a Vertex AI Gemini call (5-60s, expensive)
# and we don't want to fan out so wide that bursts blow past Vertex quotas
# or balloon costs. The worker is idempotent so retries on transient backend
# errors are safe; AI-call failures are persisted (no retry needed).
resource "google_cloud_tasks_queue" "verify" {
  project  = var.project_id
  location = var.region
  name     = "fastfso-${var.environment}-verify"

  rate_limits {
    max_dispatches_per_second = 10
    max_concurrent_dispatches = 5
  }

  stackdriver_logging_config {
    sampling_ratio = var.task_log_sampling_ratio
  }

  retry_config {
    max_attempts       = 5
    max_retry_duration = "3600s"
    min_backoff        = "1s"
    max_backoff        = "60s"
    max_doublings      = 4
  }
}

# Cloud Run v2 service
resource "google_cloud_run_v2_service" "backend" {
  project              = var.project_id
  name                 = "fastfso-${var.environment}-backend"
  location             = var.region
  deletion_protection  = var.deletion_protection
  ingress              = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
  invoker_iam_disabled = true

  # client / client_version are set by gcloud. The container image is rolled
  # forward by the CI/CD deploy pipeline — ignore here so `terraform apply`
  # does not revert prod to the Terraform-default image.
  lifecycle {
    ignore_changes = [
      client,
      client_version,
      template[0].containers[0].image,
    ]
  }

  scaling {
    manual_instance_count = 0
    min_instance_count    = var.min_instance_count
  }

  template {
    service_account = google_service_account.backend.email

    scaling {
      min_instance_count = var.min_instance_count
      max_instance_count = var.max_instance_count
    }

    vpc_access {
      network_interfaces {
        network    = var.network_id
        subnetwork = var.subnet_id
      }
      egress = "PRIVATE_RANGES_ONLY"
    }

    containers {
      name  = "backend"
      image = var.container_image

      depends_on = ["clamav"]

      ports {
        container_port = 8080
      }

      # Bill CPU only while a request is in flight. Without this the provider
      # defaults to cpu_idle=false ("CPU always allocated"), which billed prod's
      # min_instance_count=1 instance at the full $0.000018/vCPU-s around the
      # clock -- ~$106/mo to serve ~19 human requests. The idle rate is
      # $0.0000025/vCPU-s, 7.2x cheaper.
      resources {
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }
        cpu_idle = true
      }

      env {
        name  = "CLAMAV_ADDR"
        value = "127.0.0.1:3310"
      }

      env {
        name  = "DB_HOST"
        value = var.db_private_ip
      }

      env {
        name  = "DB_PORT"
        value = "5432"
      }

      env {
        name  = "DB_NAME"
        value = var.db_name
      }

      env {
        name  = "DB_USER"
        value = var.db_user
      }

      env {
        name = "DB_PASSWORD"
        value_source {
          secret_key_ref {
            secret  = var.db_password_secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "SENDGRID_API_KEY"
        value_source {
          secret_key_ref {
            secret  = var.sendgrid_api_key_secret_id
            version = "latest"
          }
        }
      }

      env {
        name  = "APP_ENV"
        value = var.environment
      }

      env {
        name  = "DEPLOY_ENV"
        value = "cloud"
      }

      env {
        name  = "FRONTEND_URL"
        value = var.frontend_url
      }

      env {
        name  = "BACKEND_URL"
        value = var.backend_url
      }

      env {
        name  = "GIN_MODE"
        value = "release"
      }

      env {
        name  = "EMAIL_CLOUD_TASKS_QUEUE"
        value = google_cloud_tasks_queue.email.id
      }

      env {
        name  = "VERIFY_CLOUD_TASKS_QUEUE"
        value = google_cloud_tasks_queue.verify.id
      }

      env {
        name  = "CLOUD_TASKS_LOCATION"
        value = var.region
      }

      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }

      env {
        name  = "VERTEX_LOCATION"
        value = var.region
      }

      env {
        name  = "BACKEND_SERVICE_ACCOUNT"
        value = google_service_account.backend.email
      }

      env {
        name  = "STORAGE_BUCKET"
        value = var.storage_bucket
      }
    }

    # ClamAV sidecar — scans user-uploaded files on the backend upload
    # paths (task, travel, wiki) before they are written to Cloud Storage.
    # clamd listens on 3310; the backend reaches it via the shared
    # network namespace at 127.0.0.1:3310.
    containers {
      name  = "clamav"
      image = var.clamav_image

      # Must match the backend container: Cloud Run throttles CPU per instance,
      # not per container, and rejects a mixed cpu_idle across containers.
      #
      # clamd does a background SelfCheck every ~10 min and freshclam refreshes
      # signatures on a timer; both get throttled between requests now. Signature
      # loading during startup is unaffected (startup CPU is always allocated),
      # and clamd gets full CPU whenever the backend is actually serving an
      # upload. Watch for stale-signature warnings in the clamav logs.
      resources {
        limits = {
          cpu    = var.clamav_cpu
          memory = var.clamav_memory
        }
        cpu_idle = true
      }

      # clamd loads signatures on startup (~30-60s). Cloud Run must not
      # route to the backend until clamd is accepting connections.
      startup_probe {
        tcp_socket {
          port = 3310
        }
        initial_delay_seconds = 10
        period_seconds        = 5
        timeout_seconds       = 3
        failure_threshold     = 30
      }
    }
  }
}

# Cloud Run job for database migrations
resource "google_cloud_run_v2_job" "migrate" {
  project             = var.project_id
  name                = "fastfso-${var.environment}-migrate"
  location            = var.region
  deletion_protection = var.deletion_protection

  lifecycle {
    ignore_changes = [
      client,
      client_version,
      template[0].template[0].containers[0].image,
    ]
  }

  template {
    task_count = 1

    template {
      service_account = google_service_account.backend.email
      max_retries     = 0
      timeout         = "300s"

      vpc_access {
        network_interfaces {
          network    = var.network_id
          subnetwork = var.subnet_id
        }
        egress = "PRIVATE_RANGES_ONLY"
      }

      containers {
        image   = var.container_image
        command = ["/bin/fastfso", "migrate", "up"]

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        env {
          name  = "DB_HOST"
          value = var.db_private_ip
        }

        env {
          name  = "DB_PORT"
          value = "5432"
        }

        env {
          name  = "DB_NAME"
          value = var.db_name
        }

        env {
          name  = "DB_USER"
          value = var.db_user
        }

        env {
          name = "DB_PASSWORD"
          value_source {
            secret_key_ref {
              secret  = var.db_password_secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }
}

# Cloud Run job for database seeding
resource "google_cloud_run_v2_job" "seed" {
  project             = var.project_id
  name                = "fastfso-${var.environment}-seed"
  location            = var.region
  deletion_protection = var.deletion_protection

  lifecycle {
    ignore_changes = [
      client,
      client_version,
      template[0].template[0].containers[0].image,
    ]
  }

  template {
    task_count = 1

    template {
      service_account = google_service_account.backend.email
      max_retries     = 0
      timeout         = "300s"

      vpc_access {
        network_interfaces {
          network    = var.network_id
          subnetwork = var.subnet_id
        }
        egress = "PRIVATE_RANGES_ONLY"
      }

      containers {
        image   = var.container_image
        command = ["/bin/fastfso", "seed"]

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        env {
          name  = "DB_HOST"
          value = var.db_private_ip
        }

        env {
          name  = "DB_PORT"
          value = "5432"
        }

        env {
          name  = "DB_NAME"
          value = var.db_name
        }

        env {
          name  = "DB_USER"
          value = var.db_user
        }

        env {
          name = "DB_PASSWORD"
          value_source {
            secret_key_ref {
              secret  = var.db_password_secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }
}

# Cloud Scheduler: nightly session cleanup
resource "google_cloud_scheduler_job" "session_cleanup" {
  project     = var.project_id
  region      = var.region
  name        = "fastfso-${var.environment}-session-cleanup"
  description = "Purge expired sessions"
  schedule    = "0 3 * * *" # 03:00 UTC daily
  time_zone   = "UTC"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${var.backend_url}/api/cron/session-cleanup"

    oidc_token {
      service_account_email = google_service_account.backend.email
      audience              = var.backend_url
    }
  }
}

# Cloud Scheduler: daily summary digest emails
resource "google_cloud_scheduler_job" "daily_digest" {
  project     = var.project_id
  region      = var.region
  name        = "fastfso-${var.environment}-daily-digest"
  description = "Send daily summary emails to users opted into the digest"
  schedule    = "0 13 * * *" # 13:00 UTC ≈ 9am ET daily
  time_zone   = "UTC"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${var.backend_url}/api/cron/daily-digest"

    oidc_token {
      service_account_email = google_service_account.backend.email
      audience              = var.backend_url
    }
  }
}

# Cloud Scheduler: due-date / overdue task reminders for every_task users
# (daily_summary users get the same counts in their digest at 13:00 UTC)
resource "google_cloud_scheduler_job" "task_reminders" {
  project     = var.project_id
  region      = var.region
  name        = "fastfso-${var.environment}-task-reminders"
  description = "Send overdue + due-soon reminders to ICs on per-event email"
  schedule    = "0 14 * * *" # 14:00 UTC ≈ 10am ET daily
  time_zone   = "UTC"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${var.backend_url}/api/cron/task-reminders"

    oidc_token {
      service_account_email = google_service_account.backend.email
      audience              = var.backend_url
    }
  }
}

# Cloud Scheduler: daily travel debrief detection
resource "google_cloud_scheduler_job" "travel_debrief" {
  project     = var.project_id
  region      = var.region
  name        = "fastfso-${var.environment}-travel-debrief"
  description = "Create post-travel debrief tasks for completed trips"
  schedule    = "0 8 * * *" # 08:00 UTC daily
  time_zone   = "UTC"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${var.backend_url}/api/cron/travel-debrief"

    oidc_token {
      service_account_email = google_service_account.backend.email
      audience              = var.backend_url
    }
  }
}

# Serverless NEG for the load balancer
resource "google_compute_region_network_endpoint_group" "backend" {
  project               = var.project_id
  name                  = "fastfso-${var.environment}-backend-neg"
  region                = var.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_v2_service.backend.name
  }
}
