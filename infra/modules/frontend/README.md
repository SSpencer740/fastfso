# Frontend Module

Creates a GCS bucket for hosting the React SPA.

## Resources

- GCS bucket with website configuration and CORS
- Public IAM binding for `allUsers` object viewer access

## SPA Routing

Uses `not_found_page = "index.html"` so all routes are handled by the React app.
