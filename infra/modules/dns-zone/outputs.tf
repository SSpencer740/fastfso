output "zone_name" {
  description = "Cloud DNS managed zone name"
  value       = google_dns_managed_zone.main.name
}

output "name_servers" {
  description = "Name servers to configure at the domain registrar"
  value       = google_dns_managed_zone.main.name_servers
}
