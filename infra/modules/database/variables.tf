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

variable "db_tier" {
  type        = string
  description = "Cloud SQL machine tier"
  default     = "db-f1-micro"
}

variable "db_edition" {
  type        = string
  description = "Cloud SQL edition (ENTERPRISE or ENTERPRISE_PLUS)"
  default     = "ENTERPRISE"
}

variable "db_name" {
  type        = string
  description = "Database name"
  default     = "fastfso"
}

variable "db_user" {
  type        = string
  description = "Database user name"
  default     = "fastfso"
}

variable "deletion_protection" {
  type        = bool
  description = "Block terraform destroy on the SQL instance. Defaults on; only disable for environments that are intentionally torn down (staging)."
  default     = true
}
