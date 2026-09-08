package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/SSpencer740/fastfso/backend/internal/scan"
)

// MaxUploadSize is the default per-request upload cap for attachments.
const MaxUploadSize = 50 << 20 // 50 MB

// SniffContentType reads the first 512 bytes to detect the MIME type,
// then rewinds the reader so the caller can stream the full content.
// The passed file must implement io.ReadSeeker (multipart.File does).
func SniffContentType(file io.ReadSeeker) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read for sniff: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind after sniff: %w", err)
	}
	return http.DetectContentType(buf[:n]), nil
}

// ScanAndRewind runs the given scanner over the full content of file,
// rewinding the reader afterwards so the caller can stream it elsewhere
// (e.g. to object storage). Returns the scanner's error unchanged, which
// callers can inspect for *scan.InfectedError via errors.As.
func ScanAndRewind(ctx context.Context, scanner scan.Scanner, file io.ReadSeeker) error {
	if err := scanner.Scan(ctx, file); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind after scan: %w", err)
	}
	return nil
}
