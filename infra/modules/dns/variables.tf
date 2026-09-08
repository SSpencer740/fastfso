variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "dns_zone_name" {
  type        = string
  description = "Cloud DNS managed zone name (from global environment)"
}

variable "domain" {
  type        = string
  description = "Domain name for DNS records"
}

variable "lb_static_ip" {
  type        = string
  description = "Load balancer static IP address"
}

variable "create_www_cname" {
  type        = bool
  description = "Whether to create a www CNAME record"
  default     = false
}
