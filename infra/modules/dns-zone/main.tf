# Public DNS zone
resource "google_dns_managed_zone" "main" {
  project     = var.project_id
  name        = "fastfso-${replace(var.base_domain, ".", "-")}"
  dns_name    = "${var.base_domain}."
  description = "DNS zone for ${var.base_domain}"

  dnssec_config {
    state = "on"
  }
}

# CAA record restricting cert issuance to Google's CA
resource "google_dns_record_set" "caa" {
  project      = var.project_id
  name         = "${var.base_domain}."
  type         = "CAA"
  ttl          = 300
  managed_zone = google_dns_managed_zone.main.name
  rrdatas = [
    "0 issue \"pki.goog\"",
    "0 issue \"letsencrypt.org\"",
  ]
}
