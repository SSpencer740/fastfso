variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "region" {
  type        = string
  description = "GCP region"
  default     = "us-east4"
}

variable "environment" {
  type        = string
  description = "Environment name"
  default     = "prod"
}

variable "base_domain" {
  type        = string
  description = "Base domain name (e.g. cloud.fastfso.com)"
  default     = "cloud.fastfso.com"
}

variable "container_image" {
  type        = string
  description = "Container image to deploy to Cloud Run"
}

variable "public_domain" {
  type        = string
  description = "Public-facing domain (e.g. app.fastfso.com) — used for FRONTEND_URL/BACKEND_URL and added to LB host rules"
  default     = ""
}

variable "db_tier" {
  type        = string
  description = "Cloud SQL machine tier"
  # Sized for current load (pilot/pre-GA), not headroom. Scale up before
  # onboarding a real tenant — a tier change restarts the instance, so do it
  # during a maintenance window once there are users to disrupt.
  default = "db-custom-1-3840"
}

variable "max_instance_count" {
  type        = number
  description = "Maximum number of Cloud Run instances"
  default     = 10
}

variable "min_instance_count" {
  type        = number
  description = "Minimum number of Cloud Run instances (keep warm to avoid cold starts)"
  default     = 1
}
