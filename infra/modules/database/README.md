# Database Module

Creates a private Cloud SQL PostgreSQL 16 instance with automatic password management.

## Resources

- Private IP range for VPC peering
- Private services access connection
- Cloud SQL PostgreSQL 16 instance (private IP only, SSL enforced)
- Database and user
- Auto-generated password stored in Secret Manager

## Security

- No public IP — only accessible via VPC peering
- SSL required for all connections
- Password auto-generated (never passed as a variable)
- Password stored in Secret Manager, referenced by Cloud Run via `secret_key_ref`
