variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "region" {
  type        = string
  description = "GCP region"
}

variable "environment" {
  type        = string
  description = "Environment name (staging, prod)"
}

variable "network_id" {
  type        = string
  description = "VPC network ID"
}

variable "subnet_id" {
  type        = string
  description = "Subnet ID"
}

variable "container_image" {
  type        = string
  description = "Container image to deploy"
}

variable "db_private_ip" {
  type        = string
  description = "Cloud SQL private IP address"
}

variable "db_name" {
  type        = string
  description = "Database name"
}

variable "db_user" {
  type        = string
  description = "Database user"
}

variable "db_password_secret_id" {
  type        = string
  description = "Secret Manager secret ID for the database password"
}

variable "sendgrid_api_key_secret_id" {
  type        = string
  description = "Secret Manager secret ID for the SendGrid API key"
}

variable "storage_bucket" {
  type        = string
  description = "Name of the GCS bucket for user-uploaded files"
}

variable "frontend_url" {
  type        = string
  description = "Frontend URL (e.g. https://app-staging.cloud.fastfso.com)"
}

variable "backend_url" {
  type        = string
  description = "Backend URL (e.g. https://app-staging.cloud.fastfso.com)"
}

variable "max_instance_count" {
  type        = number
  description = "Maximum number of Cloud Run instances"
  default     = 3
}

variable "min_instance_count" {
  type        = number
  description = "Minimum number of Cloud Run instances (set >0 to keep warm and avoid cold starts)"
  default     = 0
}

variable "cpu" {
  type        = string
  description = "CPU limit for Cloud Run"
  default     = "1"
}

variable "memory" {
  type        = string
  description = "Memory limit for Cloud Run"
  default     = "512Mi"
}

variable "task_log_sampling_ratio" {
  type        = number
  description = "Cloud Tasks log sampling ratio (0.0 = none, 1.0 = all)"
  default     = 0.0
}

variable "clamav_image" {
  type        = string
  description = "ClamAV container image for the malware-scanning sidecar"
  default     = "clamav/clamav:stable"
}

variable "clamav_cpu" {
  type        = string
  description = "CPU limit for the ClamAV sidecar"
  default     = "1"
}

variable "clamav_memory" {
  type        = string
  description = "Memory limit for the ClamAV sidecar. clamd keeps the full signature database in RAM; 2Gi is the practical minimum."
  default     = "2Gi"
}

variable "deletion_protection" {
  type        = bool
  description = "Block terraform destroy on the Cloud Run service and jobs. The provider defaults this to true, so it must be explicitly disabled for environments that are torn down (staging)."
  default     = true
}
