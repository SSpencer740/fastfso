# Load Balancer Module

Creates a global HTTPS load balancer with Cloud Armor WAF protection.

## Resources (12)

- Global static IP
- Google-managed SSL certificate
- Strict SSL policy (RESTRICTED, TLS 1.2+)
- Cloud Armor policy (SQLi, XSS, rate limiting)
- Health check (HTTP :8080 /health)
- Backend service (EXTERNAL_MANAGED) with Cloud Armor
- Backend bucket (CDN enabled) for frontend
- URL map (/api/* → backend, /* → frontend)
- HTTPS proxy with SSL cert and policy
- HTTPS forwarding rule (port 443)
- HTTP → HTTPS redirect (URL map + proxy + forwarding rule on port 80)

## SSL Certificate

The Google-managed SSL cert stays in `PROVISIONING` state until the DNS A record
for the domain points to the load balancer's static IP. This is expected — Terraform
apply succeeds, and the cert activates once DNS propagates.
