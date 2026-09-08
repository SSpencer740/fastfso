package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	gcs "cloud.google.com/go/storage"
)

// GCS stores files in Google Cloud Storage.
type GCS struct {
	client *gcs.Client
	bucket string
}

// NewGCS creates a GCS storage backend for the given bucket.
func NewGCS(client *gcs.Client, bucket string) *GCS {
	return &GCS{client: client, bucket: bucket}
}

func (g *GCS) Upload(ctx context.Context, key string, r io.Reader, contentType string) error {
	w := g.client.Bucket(g.bucket).Object(key).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return fmt.Errorf("upload to GCS: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close GCS writer: %w", err)
	}
	return nil
}

func (g *GCS) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	r, err := g.client.Bucket(g.bucket).Object(key).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("download from GCS: %w", err)
	}
	return r, nil
}

func (g *GCS) Delete(ctx context.Context, key string) error {
	if err := g.client.Bucket(g.bucket).Object(key).Delete(ctx); err != nil {
		return fmt.Errorf("delete from GCS: %w", err)
	}
	return nil
}

func (g *GCS) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	url, err := g.client.Bucket(g.bucket).SignedURL(key, &gcs.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(ttl),
		// Override response disposition so browsers render PDFs/images
		// inline (and admin <embed> previews work). Matches the local
		// storage behavior. Users can still save via right-click.
		QueryParameters: map[string][]string{
			"response-content-disposition": {"inline"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("sign GCS URL: %w", err)
	}
	return url, nil
}
