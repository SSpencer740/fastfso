# Networking Module

Creates the VPC, subnet, and firewall rules for fastFSO.

## Resources

- Custom VPC with no auto-created subnets
- Regional subnet with Private Google Access and flow logs
- Firewall rules: health checks, internal, IAP SSH, deny-all default

## No Cloud NAT

Cloud Run uses `PRIVATE_RANGES_ONLY` egress — VPC for Cloud SQL, default internet
gateway for external calls. Add Cloud NAT later if GCE instances are introduced.
