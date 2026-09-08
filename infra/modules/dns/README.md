# DNS Module

Creates DNS records in the shared Cloud DNS zone for a per-environment app domain.

## Resources

- A record for app domain → load balancer IP
- CNAME record for www → app domain (optional, via `create_www_cname`)

## Prerequisites

The DNS zone must already exist (created by the `global` environment via the `dns-zone` module). Pass the zone name as `dns_zone_name`.
