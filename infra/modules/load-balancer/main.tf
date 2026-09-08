# Static IP for the load balancer
resource "google_compute_global_address" "lb" {
  project = var.project_id
  name    = "fastfso-${var.environment}-lb-ip"
}

# Google-managed SSL certificate (only when not using shared cert map)
resource "google_compute_managed_ssl_certificate" "lb" {
  count   = var.certificate_map_id == "" ? 1 : 0
  project = var.project_id
  name    = "fastfso-${var.environment}-ssl-cert"

  managed {
    domains = concat([var.domain], var.additional_domains)
  }
}

# Strict SSL policy — TLS 1.2+ with RESTRICTED profile
resource "google_compute_ssl_policy" "lb" {
  project         = var.project_id
  name            = "fastfso-${var.environment}-ssl-policy"
  profile         = "RESTRICTED"
  min_tls_version = "TLS_1_2"
}

# Cloud Armor security policy
resource "google_compute_security_policy" "waf" {
  project = var.project_id
  name    = "fastfso-${var.environment}-waf"

  # Default rule: allow
  rule {
    action   = "allow"
    priority = 2147483647

    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }

    description = "Default allow"
  }

  # SQLi protection. Skipped on:
  # - Internal task paths (authenticated via OIDC)
  # - Multipart form uploads — PDF/image byte streams contain sequences that
  #   look like SQL injection to libinjection (e.g. rule 942100), causing
  #   false-positive 403s on legitimate file uploads. Uploads are otherwise
  #   protected: auth + CSRF + ClamAV scan + our app uses parameterized
  #   queries so form-field SQLi can't reach the DB.
  rule {
    action   = "deny(403)"
    priority = 1000

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('sqli-v33-stable', {'sensitivity': 1}) && !request.path.startsWith('/api/tasks/') && !request.headers['content-type'].lower().startsWith('multipart/form-data')"
      }
    }

    description = "Block SQL injection"
  }

  # XSS protection. Same exclusions as SQLi above and for the same reasons:
  # the OWASP XSS rule scans binary form bodies and false-positives on PDFs.
  rule {
    action   = "deny(403)"
    priority = 1001

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('xss-v33-stable', {'sensitivity': 1}) && !request.path.startsWith('/api/tasks/') && !request.headers['content-type'].lower().startsWith('multipart/form-data')"
      }
    }

    description = "Block XSS"
  }

  # Local file inclusion
  rule {
    action   = "deny(403)"
    priority = 1010

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('lfi-v33-stable', {'sensitivity': 1})"
      }
    }

    description = "Block local file inclusion"
  }

  # Remote code execution
  rule {
    action   = "deny(403)"
    priority = 1011

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('rce-v33-stable', {'sensitivity': 1})"
      }
    }

    description = "Block remote code execution"
  }

  # Remote file inclusion
  rule {
    action   = "deny(403)"
    priority = 1012

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('rfi-v33-stable', {'sensitivity': 1})"
      }
    }

    description = "Block remote file inclusion"
  }

  # Scanner detection
  rule {
    action   = "deny(403)"
    priority = 1013

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('scannerdetection-v33-stable', {'sensitivity': 1})"
      }
    }

    description = "Block scanner traffic"
  }

  # Protocol attacks
  rule {
    action   = "deny(403)"
    priority = 1014

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('protocolattack-v33-stable', {'sensitivity': 1})"
      }
    }

    description = "Block protocol attacks"
  }

  # Session fixation
  rule {
    action   = "deny(403)"
    priority = 1015

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('sessionfixation-v33-stable', {'sensitivity': 1})"
      }
    }

    description = "Block session fixation"
  }

  # HTTP method enforcement
  rule {
    action   = "deny(403)"
    priority = 1016

    match {
      expr {
        expression = "evaluatePreconfiguredWaf('methodenforcement-v33-stable', {'sensitivity': 1})"
      }
    }

    description = "Block disallowed HTTP methods"
  }

  # Rate limiting
  rule {
    action   = "rate_based_ban"
    priority = 1002

    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }

    rate_limit_options {
      conform_action = "allow"
      exceed_action  = "deny(429)"

      rate_limit_threshold {
        count        = 100
        interval_sec = 60
      }

      ban_duration_sec = 300
    }

    description = "Rate limit 100 req/min per IP"
  }
}

# Backend service pointing to the serverless NEG
# Note: serverless NEGs do not support health checks — Cloud Run manages health internally.
resource "google_compute_backend_service" "backend" {
  project               = var.project_id
  name                  = "fastfso-${var.environment}-backend-svc"
  protocol              = "HTTPS"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  security_policy       = google_compute_security_policy.waf.id

  log_config {
    enable      = var.backend_log_sample_rate > 0
    sample_rate = var.backend_log_sample_rate
  }

  backend {
    group = var.neg_id
  }
}

# Backend bucket for the frontend SPA (CDN enabled)
resource "google_compute_backend_bucket" "frontend" {
  project     = var.project_id
  name        = "fastfso-${var.environment}-frontend-bucket"
  bucket_name = var.frontend_bucket_name
  enable_cdn  = true

  compression_mode = "AUTOMATIC"

  cdn_policy {
    cache_mode  = "FORCE_CACHE_ALL"
    default_ttl = 300 # 5 min — keeps index.html fresh
    client_ttl  = 300
  }
}

# URL map: /api/* → backend service, /* → frontend bucket
resource "google_compute_url_map" "lb" {
  project         = var.project_id
  name            = "fastfso-${var.environment}-url-map"
  default_service = google_compute_backend_bucket.frontend.id

  # Content-serving host rule: when redirect is configured, only additional_domains serve content.
  host_rule {
    hosts        = var.redirect_to != "" ? var.additional_domains : concat([var.domain], var.additional_domains)
    path_matcher = "main"
  }

  # Redirect primary domain → canonical public domain (301)
  dynamic "host_rule" {
    for_each = var.redirect_to != "" ? [1] : []
    content {
      hosts        = [var.domain]
      path_matcher = "redirect"
    }
  }

  path_matcher {
    name            = "main"
    default_service = google_compute_backend_bucket.frontend.id

    # API routes → backend Cloud Run service
    route_rules {
      priority = 1
      service  = google_compute_backend_service.backend.id
      match_rules {
        prefix_match = "/api/"
      }
    }

    # Vite hashed assets → serve directly from bucket
    route_rules {
      priority = 2
      service  = google_compute_backend_bucket.frontend.id
      match_rules {
        prefix_match = "/assets/"
      }
    }

    # SPA catch-all → rewrite to index.html so GCS returns 200
    route_rules {
      priority = 3
      service  = google_compute_backend_bucket.frontend.id
      match_rules {
        path_template_match = "/{path=**}"
      }
      route_action {
        url_rewrite {
          path_template_rewrite = "/index.html"
        }
      }
    }
  }

  dynamic "path_matcher" {
    for_each = var.redirect_to != "" ? [1] : []
    content {
      name = "redirect"
      default_url_redirect {
        host_redirect          = var.redirect_to
        redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
        strip_query            = false
      }
    }
  }
}

# HTTPS proxy
resource "google_compute_target_https_proxy" "lb" {
  project          = var.project_id
  name             = "fastfso-${var.environment}-https-proxy"
  url_map          = google_compute_url_map.lb.id
  ssl_certificates = var.certificate_map_id != "" ? null : [google_compute_managed_ssl_certificate.lb[0].id]
  certificate_map  = var.certificate_map_id != "" ? var.certificate_map_id : null
  ssl_policy       = google_compute_ssl_policy.lb.id
}

# HTTPS forwarding rule (port 443)
resource "google_compute_global_forwarding_rule" "https" {
  project               = var.project_id
  name                  = "fastfso-${var.environment}-https-rule"
  target                = google_compute_target_https_proxy.lb.id
  ip_address            = google_compute_global_address.lb.address
  port_range            = "443"
  load_balancing_scheme = "EXTERNAL_MANAGED"
}

# HTTP → HTTPS redirect
resource "google_compute_url_map" "http_redirect" {
  project = var.project_id
  name    = "fastfso-${var.environment}-http-redirect"

  default_url_redirect {
    https_redirect         = true
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
    strip_query            = false
  }
}

resource "google_compute_target_http_proxy" "http_redirect" {
  project = var.project_id
  name    = "fastfso-${var.environment}-http-proxy"
  url_map = google_compute_url_map.http_redirect.id
}

resource "google_compute_global_forwarding_rule" "http_redirect" {
  project               = var.project_id
  name                  = "fastfso-${var.environment}-http-rule"
  target                = google_compute_target_http_proxy.http_redirect.id
  ip_address            = google_compute_global_address.lb.address
  port_range            = "80"
  load_balancing_scheme = "EXTERNAL_MANAGED"
}
