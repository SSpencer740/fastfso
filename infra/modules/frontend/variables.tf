variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "environment" {
  type        = string
  description = "Environment name (staging, prod)"
}

variable "domain" {
  type        = string
  description = "Domain name for CORS configuration"
}

variable "location" {
  type        = string
  description = "GCS bucket location"
  default     = "US"
}

variable "public_access_tag_value_id" {
  type        = string
  description = "Tag value ID for the allowPublicAccess tag (from global infra)"
}

variable "force_destroy" {
  type        = bool
  description = "Allow terraform destroy to delete the bucket while it still holds objects. Defaults off so prod data cannot be dropped by a stray destroy; staging enables it because it is torn down between releases."
  default     = false
}
