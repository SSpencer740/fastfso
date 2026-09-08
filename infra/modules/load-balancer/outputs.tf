output "static_ip" {
  description = "Global static IP address for the load balancer"
  value       = google_compute_global_address.lb.address
}

output "ssl_cert_name" {
  description = "Google-managed SSL certificate name (null when using shared cert map)"
  value       = var.certificate_map_id == "" ? google_compute_managed_ssl_certificate.lb[0].name : null
}

output "cloud_armor_policy_name" {
  description = "Cloud Armor security policy name"
  value       = google_compute_security_policy.waf.name
}
