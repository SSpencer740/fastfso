resource "google_compute_network" "vpc" {
  project                 = var.project_id
  name                    = "fastfso-${var.environment}-vpc"
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"
}

resource "google_compute_subnetwork" "subnet" {
  project                  = var.project_id
  name                     = "fastfso-${var.environment}-subnet"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  ip_cidr_range            = var.subnet_cidr
  private_ip_google_access = true

  log_config {
    aggregation_interval = "INTERVAL_5_SEC"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

# Allow health check probes from Google's health check ranges
resource "google_compute_firewall" "allow_health_checks" {
  project = var.project_id
  name    = "fastfso-${var.environment}-allow-health-checks"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
    ports    = ["8080"]
  }

  source_ranges = [
    "35.191.0.0/16",
    "130.211.0.0/22",
  ]

  priority = 1000
}

# Allow internal traffic within the VPC
resource "google_compute_firewall" "allow_internal" {
  project = var.project_id
  name    = "fastfso-${var.environment}-allow-internal"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
  }

  allow {
    protocol = "udp"
  }

  allow {
    protocol = "icmp"
  }

  source_ranges = [var.subnet_cidr]

  priority = 1000
}

# Allow IAP SSH for debugging
resource "google_compute_firewall" "allow_iap_ssh" {
  project = var.project_id
  name    = "fastfso-${var.environment}-allow-iap-ssh"
  network = google_compute_network.vpc.id

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["35.235.240.0/20"]

  priority = 1000
}

# Deny all other ingress at lowest priority
resource "google_compute_firewall" "deny_all" {
  project = var.project_id
  name    = "fastfso-${var.environment}-deny-all"
  network = google_compute_network.vpc.id

  deny {
    protocol = "all"
  }

  source_ranges = ["0.0.0.0/0"]

  priority = 65534
}
