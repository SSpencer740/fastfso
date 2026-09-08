# DNS Zone Module

Creates the shared Cloud DNS zone for the fastFSO base domain with DNSSEC enabled.

## Resources

- Public DNS managed zone with DNSSEC
- CAA record restricting certificate issuance to `pki.goog` (Google's CA)

## Usage

This module is used by the `global` environment. Per-environment modules create their own DNS records in this zone via the `dns_zone_name` output.

## After Apply

Update the domain registrar's nameservers to the `name_servers` output values.
