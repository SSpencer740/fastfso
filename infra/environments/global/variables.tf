variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "base_domain" {
  type        = string
  description = "Base domain name managed in GCP (e.g. cloud.fastfso.com)"
  default     = "cloud.fastfso.com"
}

variable "root_domain" {
  type        = string
  description = "Root domain managed externally (e.g. fastfso.com)"
  default     = "fastfso.com"
}

variable "github_org" {
  type        = string
  description = "GitHub organization name"
  default     = "fastfso"
}

variable "github_repo" {
  type        = string
  description = "GitHub repository name"
  default     = "fastfso"
}

variable "region" {
  type        = string
  description = "GCP region"
  default     = "us-east4"
}