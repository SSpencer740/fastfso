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
  default     = "staging"
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
  description = "Public-facing domain (e.g. app-staging.fastfso.com) — used for FRONTEND_URL/BACKEND_URL and added to LB host rules"
  default     = ""
}

variable "db_tier" {
  type        = string
  description = "Cloud SQL machine tier"
  default     = "db-g1-small"
}
