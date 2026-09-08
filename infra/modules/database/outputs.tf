output "connection_name" {
  description = "Cloud SQL connection name"
  value       = google_sql_database_instance.postgres.connection_name
}

output "private_ip" {
  description = "Cloud SQL private IP address"
  value       = google_sql_database_instance.postgres.private_ip_address
}

output "instance_name" {
  description = "Cloud SQL instance name"
  value       = google_sql_database_instance.postgres.name
}

output "db_name" {
  description = "Database name"
  value       = google_sql_database.db.name
}

output "db_user" {
  description = "Database user"
  value       = google_sql_user.user.name
}

output "db_password_secret_id" {
  description = "Secret Manager secret resource ID for the database password"
  value       = google_secret_manager_secret.db_password.secret_id
}
