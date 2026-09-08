variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "environment" {
  type        = string
  description = "Environment name (staging, prod)"
}

variable "location" {
  type        = string
  description = "GCS bucket location. Must match the backend region for low-latency reads/writes."
}

variable "backend_service_account_email" {
  type        = string
  description = "Email of the Cloud Run backend service account that reads and writes upload objects"
}

variable "force_destroy" {
  type        = bool
  description = "Allow terraform destroy to delete the bucket while it still holds objects. Defaults off so prod data cannot be dropped by a stray destroy; staging enables it because it is torn down between releases."
  default     = false
}
