package scan

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClamd accepts a single INSTREAM upload and replies with the
// pre-configured response. It records the reassembled payload so tests can
// assert what the client sent.
type fakeClamd struct {
	listener net.Listener
	response string
	wg       sync.WaitGroup

	mu       sync.Mutex
	received []byte
}

func startFakeClamd(t *testing.T, response string) *fakeClamd {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeClamd{listener: ln, response: response}
	f.wg.Add(1)
	go f.serve()
	t.Cleanup(func() {
		_ = ln.Close()
		f.wg.Wait()
	})
	return f
}

func (f *fakeClamd) Addr() string { return f.listener.Addr().String() }

func (f *fakeClamd) serve() {
	defer f.wg.Done()
	conn, err := f.listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Read "zINSTREAM\0".
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return
	}

	var payload []byte
	szBuf := make([]byte, 4)
	for {
		if _, err := io.ReadFull(conn, szBuf); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(szBuf)
		if n == 0 {
			break
		}
		chunk := make([]byte, n)
		if _, err := io.ReadFull(conn, chunk); err != nil {
			return
		}
		payload = append(payload, chunk...)
	}

	f.mu.Lock()
	f.received = payload
	f.mu.Unlock()

	_, _ = conn.Write(append([]byte(f.response), 0))
}

func TestClamAV_Scan_Clean(t *testing.T) {
	f := startFakeClamd(t, "stream: OK")
	s := NewClamAV(f.Addr(), time.Second)

	err := s.Scan(context.Background(), strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("expected clean scan, got %v", err)
	}

	f.mu.Lock()
	got := string(f.received)
	f.mu.Unlock()
	if got != "hello world" {
		t.Fatalf("fake clamd received %q, want %q", got, "hello world")
	}
}

func TestClamAV_Scan_Infected(t *testing.T) {
	f := startFakeClamd(t, "stream: Win.Test.EICAR_HDB-1 FOUND")
	s := NewClamAV(f.Addr(), time.Second)

	err := s.Scan(context.Background(), strings.NewReader("eicar-payload"))
	var infected *InfectedError
	if !errors.As(err, &infected) {
		t.Fatalf("expected InfectedError, got %v", err)
	}
	if infected.Signature != "Win.Test.EICAR_HDB-1" {
		t.Fatalf("signature = %q, want Win.Test.EICAR_HDB-1", infected.Signature)
	}
}

func TestClamAV_Scan_DialFailure(t *testing.T) {
	// Listen then immediately close to reserve a port that nothing is on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	s := NewClamAV(addr, 200*time.Millisecond)
	err = s.Scan(context.Background(), strings.NewReader("anything"))
	if err == nil {
		t.Fatal("expected dial error, got nil")
	}
	var infected *InfectedError
	if errors.As(err, &infected) {
		t.Fatalf("should not be an InfectedError: %v", err)
	}
}

func TestClamAV_Scan_UnexpectedResponse(t *testing.T) {
	f := startFakeClamd(t, "garbage")
	s := NewClamAV(f.Addr(), time.Second)

	err := s.Scan(context.Background(), strings.NewReader("x"))
	if err == nil || !strings.Contains(err.Error(), "unexpected clamd response") {
		t.Fatalf("want unexpected-response error, got %v", err)
	}
}
