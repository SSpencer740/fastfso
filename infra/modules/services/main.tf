locals {
  apis = toset([
    "compute.googleapis.com",
    "sqladmin.googleapis.com",
    "servicenetworking.googleapis.com",
    "run.googleapis.com",
    "iam.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "secretmanager.googleapis.com",
    "artifactregistry.googleapis.com",
    "iamcredentials.googleapis.com",
    "dns.googleapis.com",
    "domains.googleapis.com",
    "certificatemanager.googleapis.com",
    "orgpolicy.googleapis.com",
    "clouderrorreporting.googleapis.com",
    "cloudtasks.googleapis.com",
    "cloudscheduler.googleapis.com",
    "aiplatform.googleapis.com",
    "containeranalysis.googleapis.com",
    "containerscanning.googleapis.com",
  ])
}

resource "google_project_service" "api" {
  for_each = local.apis

  project = var.project_id
  service = each.value

  disable_on_destroy = false
}
