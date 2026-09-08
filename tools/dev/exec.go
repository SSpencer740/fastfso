package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// runPassthrough runs cmd with SIGINT/SIGTERM ignored in the parent process.
// The child receives signals directly from the terminal (shared process group)
// and handles them itself (e.g., terraform's cancel prompt, go test cleanup).
func runPassthrough(cmd *exec.Cmd) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	return cmd.Run()
}

// run executes a command with stdout/stderr connected to the terminal.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runPassthrough(cmd)
}
