# A record pointing to the load balancer
resource "google_dns_record_set" "root" {
  project      = var.project_id
  name         = "${var.domain}."
  type         = "A"
  ttl          = 300
  managed_zone = var.dns_zone_name
  rrdatas      = [var.lb_static_ip]
}

# CNAME for www → root (only needed for prod)
resource "google_dns_record_set" "www" {
  count = var.create_www_cname ? 1 : 0

  project      = var.project_id
  name         = "www.${var.domain}."
  type         = "CNAME"
  ttl          = 300
  managed_zone = var.dns_zone_name
  rrdatas      = ["${var.domain}."]
}
