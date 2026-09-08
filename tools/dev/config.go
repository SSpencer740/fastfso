package main

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	containerName = "fastfso-postgres"
	volumeName    = "fastfso-pgdata"
	postgresImage = "postgres:17"
	dbDefaultPort = 5433
)

type dbConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	External bool // true = DB managed externally, no Docker
}

var cfg dbConfig // populated in main() before command dispatch

func loadDBConfig() dbConfig {
	// Full URL takes highest priority.
	if rawURL := os.Getenv("DATABASE_URL"); rawURL != "" {
		return parseDBURL(rawURL)
	}

	c := dbConfig{
		User:     envOr("DB_USER", "fastfso"),
		Password: envOr("DB_PASSWORD", "fastfso"),
		Name:     envOr("DB_NAME", "fastfso"),
		SSLMode:  envOr("DB_SSLMODE", "disable"),
	}

	if host := os.Getenv("DB_HOST"); host != "" {
		c.Host = host
		c.Port = envOr("DB_PORT", "5432")
		c.External = true
	} else {
		// Docker mode: detect port from running container, fall back to default.
		c.Host = "localhost"
		if p, err := containerPort(); err == nil {
			c.Port = p
		} else {
			c.Port = strconv.Itoa(dbDefaultPort)
		}
	}
	return c
}

func parseDBURL(rawURL string) dbConfig {
	u, err := url.Parse(rawURL)
	if err != nil {
		return dbConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "fastfso",
			Password: "fastfso",
			Name:     "fastfso",
			SSLMode:  "disable",
			External: true,
		}
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	user := u.User.Username()
	password, _ := u.User.Password()
	name := strings.TrimPrefix(u.Path, "/")
	sslmode := u.Query().Get("sslmode")
	if sslmode == "" {
		sslmode = "disable"
	}

	return dbConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Name:     name,
		SSLMode:  sslmode,
		External: true,
	}
}

func (c dbConfig) URL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

// WithName returns a copy of the config with a different database name.
func (c dbConfig) WithName(name string) dbConfig {
	c.Name = name
	return c
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// containerPort reads the host port mapped to 5432/tcp from the container config.
// Uses HostConfig.PortBindings so it works even when the container is stopped.
func containerPort() (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command("docker", "inspect", "--format",
		`{{(index (index .HostConfig.PortBindings "5432/tcp") 0).HostPort}}`,
		containerName)
	cmd.Stdout = &buf
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not read container port: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

// findAvailablePort probes ports starting from startPort, returning the first
// one that is not in use. Tries up to 10 ports.
func findAvailablePort(startPort int) (int, error) {
	for port := startPort; port < startPort+10; port++ {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, startPort+9)
}

// repoRoot returns the repository root directory. It checks FASTFSO_ROOT first,
// then walks up from the current directory looking for a .git/ directory.
func repoRoot() (string, error) {
	if root := os.Getenv("FASTFSO_ROOT"); root != "" {
		return root, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}

	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repo root (no .git/ found)")
		}
		dir = parent
	}
}
