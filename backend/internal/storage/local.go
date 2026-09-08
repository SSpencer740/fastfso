package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Local stores files on the local filesystem under a base directory.
type Local struct {
	BaseDir string
}

// NewLocal creates a Local storage backend writing to the given directory.
func NewLocal(baseDir string) *Local {
	return &Local{BaseDir: baseDir}
}

func (l *Local) Upload(ctx context.Context, key string, r io.Reader, contentType string) error {
	path := filepath.Join(l.BaseDir, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (l *Local) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(l.BaseDir, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

func (l *Local) Delete(ctx context.Context, key string) error {
	path := filepath.Join(l.BaseDir, key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (l *Local) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	// In dev mode, return a direct download path served by the backend
	return fmt.Sprintf("/api/v1/tasks/files/%s", key), nil
}
