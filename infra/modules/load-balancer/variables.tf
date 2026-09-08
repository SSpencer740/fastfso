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
  description = "Domain name for SSL certificate and URL map"
}

variable "neg_id" {
  type        = string
  description = "Serverless NEG ID for the backend"
}

variable "frontend_bucket_name" {
  type        = string
  description = "GCS bucket name for frontend assets"
}

variable "additional_domains" {
  type        = list(string)
  description = "Additional domains for SSL certificate and URL map host rules"
  default     = []
}

variable "certificate_map_id" {
  type        = string
  description = "Certificate Manager certificate map ID (when set, skips per-env SSL cert creation)"
  default     = ""
}

variable "redirect_to" {
  type        = string
  description = "When set, redirects var.domain to this host (301). Additional domains serve content."
  default     = ""
}

variable "backend_log_sample_rate" {
  type        = number
  description = "Backend service log sampling rate (0.0 = none, 1.0 = all)"
  default     = 0.0
}
