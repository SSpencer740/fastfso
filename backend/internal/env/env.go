package env

import (
	"net/url"
	"os"
	"strconv"
)

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// App

func AppEnv() string     { return get("APP_ENV", "local") }
func DeployEnv() string  { return get("DEPLOY_ENV", "local") }
func IsProduction() bool { return AppEnv() == "prod" }
func IsCloud() bool      { return DeployEnv() == "cloud" }
func Port() string       { return get("PORT", "8080") }

// Auth

func SessionSecret() string { return get("SESSION_SECRET", "dev-secret-do-not-use-in-production") }

// Database

func DatabaseURL() string { return get("DATABASE_URL", "") }
func DBHost() string      { return get("DB_HOST", "") }
func DBPort() string      { return get("DB_PORT", "") }
func DBUser() string      { return get("DB_USER", "") }
func DBPassword() string  { return get("DB_PASSWORD", "") }
func DBName() string      { return get("DB_NAME", "") }
func DBSSLMode() string   { return get("DB_SSLMODE", "require") }

// DBMaxConns caps the per-process pgxpool size. Total Postgres connections at
// peak is approximately `cloud_run_max_instances * DBMaxConns` — keep that
// product comfortably below the Cloud SQL connection limit.
func DBMaxConns() int { return getInt("DB_MAX_CONNS", 5) }

// DBMinConns is the number of idle connections each pgxpool keeps open. 0
// minimizes Cloud SQL connection pressure at the cost of cold-start latency
// on first query; bump to 1 if request-path tail latency becomes a concern.
func DBMinConns() int { return getInt("DB_MIN_CONNS", 0) }

// WebAuthn

func WebAuthnRPID() string {
	if v := os.Getenv("WEBAUTHN_RP_ID"); v != "" {
		return v
	}
	if u, err := url.Parse(FrontendURL()); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return "localhost"
}

func WebAuthnRPOrigin() string {
	if v := os.Getenv("WEBAUTHN_RP_ORIGIN"); v != "" {
		return v
	}
	if fe := FrontendURL(); fe != "" {
		if u, err := url.Parse(fe); err == nil && u.Scheme != "" && u.Host != "" {
			return u.Scheme + "://" + u.Host
		}
	}
	return "http://localhost:5173"
}

// Email & Cloud Tasks

func SendGridAPIKey() string        { return get("SENDGRID_API_KEY", "") }
func EmailFrom() string             { return get("EMAIL_FROM", "noreply@fastfso.com") }
func EmailCloudTasksQueue() string  { return get("EMAIL_CLOUD_TASKS_QUEUE", "") }
func VerifyCloudTasksQueue() string { return get("VERIFY_CLOUD_TASKS_QUEUE", "") }
func CloudTasksLocation() string    { return get("CLOUD_TASKS_LOCATION", "") }
func GCPProjectID() string          { return get("GCP_PROJECT_ID", "") }
func BackendServiceAccount() string { return get("BACKEND_SERVICE_ACCOUNT", "") }

// URLs

func FrontendURL() string { return get("FRONTEND_URL", "http://localhost:5173") }
func BackendURL() string  { return get("BACKEND_URL", "http://localhost:8080") }

// Storage

func StorageBucket() string { return get("STORAGE_BUCKET", "") }

// Vertex AI / Gemini

func VertexLocation() string { return get("VERTEX_LOCATION", "us-central1") }
func GeminiModel() string    { return get("GEMINI_MODEL", "gemini-2.5-pro") }

// Malware scanning

func ClamAVAddr() string { return get("CLAMAV_ADDR", "") }

// Testing

func IntegrationDatabaseURL() string { return get("INTEGRATION_DATABASE_URL", "") }
