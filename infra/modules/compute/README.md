# Compute Module

Deploys the Go backend on Cloud Run v2 with Direct VPC Egress.

## Resources

- Dedicated service account with minimal IAM roles
- Artifact Registry Docker repository
- Cloud Run v2 service with VPC egress and LB-only ingress
- Serverless NEG for load balancer integration

## Portability

The load balancer module consumes only `neg_id`. Swapping to GCE MIG later
requires changing only this module's internals.
