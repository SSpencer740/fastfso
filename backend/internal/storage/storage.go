package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// StorageBackend abstracts file storage for task uploads.
type StorageBackend interface {
	Upload(ctx context.Context, key string, r io.Reader, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// KeyPrefix returns the storage key prefix for a tenant+task combination.
func KeyPrefix(tenantID, taskID uuid.UUID) string {
	return fmt.Sprintf("tenants/%s/tasks/%s", tenantID, taskID)
}

// TravelKeyPrefix returns the storage key prefix for a tenant+travel report combination.
func TravelKeyPrefix(tenantID, reportID uuid.UUID) string {
	return fmt.Sprintf("tenants/%s/travel/%s", tenantID, reportID)
}

// WikiKeyPrefix returns the storage key prefix for a tenant+wiki post combination.
func WikiKeyPrefix(tenantID, postID uuid.UUID) string {
	return fmt.Sprintf("tenants/%s/wiki/%s", tenantID, postID)
}

// DD254KeyPrefix returns the storage key prefix for a tenant+DD254 combination.
func DD254KeyPrefix(tenantID, dd254ID uuid.UUID) string {
	return fmt.Sprintf("tenants/%s/dd254/%s", tenantID, dd254ID)
}
