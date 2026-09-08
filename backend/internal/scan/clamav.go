// Package scan provides virus/malware scanning of user-uploaded content
// against a ClamAV daemon (clamd) over its INSTREAM TCP protocol.
package scan

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// ErrInfected is returned when clamd reports the stream as malicious.
// The embedded Signature names the detection (e.g. "Win.Test.EICAR_HDB-1").
type InfectedError struct {
	Signature string
}

func (e *InfectedError) Error() string {
	return fmt.Sprintf("file contains malicious content: %s", e.Signature)
}

// Scanner scans streams via a clamd server.
type Scanner interface {
	Scan(ctx context.Context, r io.Reader) error
}

// Noop is a Scanner that accepts everything. Intended for local development
// only — production deployments must wire a real ClamAV backend.
type Noop struct{}

func (Noop) Scan(context.Context, io.Reader) error { return nil }

// ClamAV is a Scanner backed by a clamd TCP endpoint.
type ClamAV struct {
	Addr    string
	Timeout time.Duration
}

// NewClamAV returns a Scanner targeting addr (host:port). Zero timeout
// defaults to 60s — ClamAV streaming scans can be slow for large files.
func NewClamAV(addr string, timeout time.Duration) *ClamAV {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &ClamAV{Addr: addr, Timeout: timeout}
}

// Scan streams r to clamd using the INSTREAM command. Returns nil if the
// stream is clean, an *InfectedError if a signature matched, or another
// error on protocol/transport failure.
//
// clamd INSTREAM wire format:
//
//	-> "zINSTREAM\0"
//	-> <uint32 big-endian chunk length> <chunk bytes>  (repeat)
//	-> <uint32 0>                                      (terminator)
//	<- "stream: OK\0"  or  "stream: <sig> FOUND\0"
func (c *ClamAV) Scan(ctx context.Context, r io.Reader) error {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(c.Timeout)
	}

	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return fmt.Errorf("scan: dial clamd: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(deadline)

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return fmt.Errorf("scan: write command: %w", err)
	}

	// clamd's StreamMaxLength defaults to 25 MiB; chunk well below that.
	buf := make([]byte, 64*1024)
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			var sz [4]byte
			binary.BigEndian.PutUint32(sz[:], uint32(n))
			if _, err := conn.Write(sz[:]); err != nil {
				return fmt.Errorf("scan: write chunk size: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return fmt.Errorf("scan: write chunk: %w", err)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("scan: read source: %w", rerr)
		}
	}

	// End-of-stream marker: 4 zero bytes.
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("scan: write terminator: %w", err)
	}

	// Response is a single null-terminated string.
	respBuf := make([]byte, 0, 256)
	tmp := make([]byte, 256)
	for {
		n, rerr := conn.Read(tmp)
		if n > 0 {
			respBuf = append(respBuf, tmp[:n]...)
			if idx := indexByte(respBuf, 0); idx >= 0 {
				respBuf = respBuf[:idx]
				break
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("scan: read response: %w", rerr)
		}
	}

	resp := strings.TrimSpace(string(respBuf))
	switch {
	case resp == "stream: OK":
		return nil
	case strings.HasSuffix(resp, " FOUND"):
		sig := strings.TrimSuffix(strings.TrimPrefix(resp, "stream: "), " FOUND")
		return &InfectedError{Signature: sig}
	case strings.HasPrefix(resp, "stream: ") && strings.HasSuffix(resp, " ERROR"):
		return fmt.Errorf("scan: clamd reported error: %s", resp)
	case resp == "":
		return errors.New("scan: empty response from clamd")
	default:
		return fmt.Errorf("scan: unexpected clamd response: %q", resp)
	}
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}
