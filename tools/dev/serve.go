package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

// rawWriter converts \n to \r\n for correct display in raw terminal mode.
type rawWriter struct {
	out io.Writer
}

func (w *rawWriter) Write(p []byte) (int, error) {
	data := bytes.ReplaceAll(p, []byte("\r\n"), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	_, err := w.out.Write(data)
	return len(p), err
}

// prefixWriter wraps an io.Writer, prepending a prefix to each line of output.
// It uses a shared mutex to prevent interleaved output from concurrent writers.
type prefixWriter struct {
	out    io.Writer
	prefix string
	mu     *sync.Mutex
	buf    []byte
}

func newPrefixWriter(out io.Writer, prefix string, mu *sync.Mutex) *prefixWriter {
	return &prefixWriter{out: out, prefix: prefix, mu: mu}
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := w.buf[:idx+1]
		w.buf = w.buf[idx+1:]
		fmt.Fprintf(w.out, "%s%s", w.prefix, line)
	}
	return len(p), nil
}

// flush writes any remaining buffered data that doesn't end with a newline.
func (w *prefixWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		fmt.Fprintf(w.out, "%s%s\n", w.prefix, w.buf)
		w.buf = nil
	}
}

func cmdServe() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	// Try to enter raw terminal mode for keystroke detection (Ctrl+R restart).
	fd := int(os.Stdin.Fd())
	oldState, rawErr := term.MakeRaw(fd)
	rawMode := rawErr == nil
	if rawMode {
		defer term.Restore(fd, oldState)
	}

	// In raw mode, wrap stdout/stderr to convert \n → \r\n.
	var stdout, stderr io.Writer
	if rawMode {
		stdout = &rawWriter{os.Stdout}
		stderr = &rawWriter{os.Stderr}
	} else {
		stdout = os.Stdout
		stderr = os.Stderr
	}

	backendTag := colorize(ansiCyan+ansiBold, "[backend]") + "  "
	frontendTag := colorize(ansiMagenta+ansiBold, "[frontend]") + " "

	var mu sync.Mutex
	backendOut := newPrefixWriter(stdout, backendTag, &mu)
	backendErr := newPrefixWriter(stderr, backendTag, &mu)
	frontendOut := newPrefixWriter(stdout, frontendTag, &mu)
	frontendErr := newPrefixWriter(stderr, frontendTag, &mu)

	fmt.Fprintln(stdout, colorize(ansiCyan, "==> ")+"Starting dev servers...")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, colorize(ansiDim, "  "+colorize(ansiCyan, "backend  ")+"→ http://localhost:8081"))
	fmt.Fprintln(stdout, colorize(ansiDim, "  "+colorize(ansiMagenta, "frontend ")+"→ http://localhost:5173"))
	fmt.Fprintln(stdout)
	if rawMode {
		fmt.Fprintln(stdout, colorize(ansiDim, "  press "+colorize(ansiBold, "Ctrl+R")+" to restart backend, "+colorize(ansiBold, "Ctrl+C")+" to stop"))
		fmt.Fprintln(stdout)
	}

	// Channels for coordination.
	restartCh := make(chan struct{}, 1)
	shutdownCh := make(chan struct{})

	// In raw mode, read keystrokes directly from stdin.
	if rawMode {
		go func() {
			buf := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				switch buf[0] {
				case 0x12: // Ctrl+R
					select {
					case restartCh <- struct{}{}:
					default:
					}
				case 0x03: // Ctrl+C
					select {
					case <-shutdownCh:
					default:
						close(shutdownCh)
					}
					return
				}
			}
		}()
	}

	// Handle OS signals for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	if rawMode {
		// In raw mode, Ctrl+C is handled as byte 0x03 above; only catch external SIGTERM.
		signal.Notify(sigCh, syscall.SIGTERM)
	} else {
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	}
	go func() {
		<-sigCh
		select {
		case <-shutdownCh:
		default:
			close(shutdownCh)
		}
	}()

	// Start frontend (persists across backend restarts).
	frontend := exec.Command("npm", "run", "dev")
	frontend.Dir = root + "/frontend"
	frontend.Stdout = frontendOut
	frontend.Stderr = frontendErr
	frontend.Env = append(os.Environ(), "BACKEND_PORT=8081")
	frontend.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := frontend.Start(); err != nil {
		fmt.Fprintln(stderr, colorize(ansiRed, "  ✗ ")+"failed to start frontend: "+err.Error())
		return
	}

	frontendExited := make(chan struct{})
	go func() {
		frontend.Wait()
		frontendOut.flush()
		frontendErr.flush()
		close(frontendExited)
	}()

	// killGroup sends SIGTERM to a process group, escalating to SIGKILL after 5s.
	killGroup := func(pid int, done <-chan error) {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			<-done
		}
	}

	// stopFrontend terminates the frontend process.
	stopFrontend := func() {
		_ = syscall.Kill(-frontend.Process.Pid, syscall.SIGTERM)
		select {
		case <-frontendExited:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-frontend.Process.Pid, syscall.SIGKILL)
			<-frontendExited
		}
	}

	// Backend restart loop.
	for {
		backend := exec.Command("go", "run", ".")
		backend.Dir = root + "/backend"
		backend.Stdout = backendOut
		backend.Stderr = backendErr
		env := append(os.Environ(),
			"DATABASE_URL="+cfg.URL(),
			"PORT=8081",
			"APP_ENV=local",
			"DEPLOY_ENV=local",
			"FRONTEND_URL=http://localhost:5173",
			"BACKEND_URL=http://localhost:8081",
		)
		// If the local ClamAV container is running, auto-set CLAMAV_ADDR so the
		// upload handlers exercise the scan path instead of the Noop scanner.
		if clamavContainerState() == "running" {
			if port, err := clamavHostPort(); err == nil && port != "" {
				env = append(env, "CLAMAV_ADDR=localhost:"+port)
			}
		}
		backend.Env = env
		backend.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := backend.Start(); err != nil {
			fmt.Fprintln(stderr, colorize(ansiRed, "  ✗ ")+"failed to start backend: "+err.Error())
			stopFrontend()
			return
		}

		backendPid := backend.Process.Pid
		backendDone := make(chan error, 1)
		go func() {
			backendDone <- backend.Wait()
			backendOut.flush()
			backendErr.flush()
		}()

		select {
		case <-restartCh:
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, colorize(ansiYellow, "  ⟳ ")+"restarting backend...")
			fmt.Fprintln(stdout)
			killGroup(backendPid, backendDone)
			// Drain any extra queued restarts.
			select {
			case <-restartCh:
			default:
			}
			continue

		case <-shutdownCh:
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, colorize(ansiDim, "  shutting down..."))
			killGroup(backendPid, backendDone)
			stopFrontend()
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, colorize(ansiGreen, "  ✓ ")+"goodbye 👋")
			return

		case <-frontendExited:
			killGroup(backendPid, backendDone)
			return

		case <-backendDone:
			stopFrontend()
			return
		}
	}
}
