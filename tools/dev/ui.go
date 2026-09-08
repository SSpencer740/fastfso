package main

import (
	"fmt"
	"os"
)

// ANSI escape codes.
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
)

// noColor returns true when color output should be suppressed.
// Respects the NO_COLOR convention (https://no-color.org/).
func noColor() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}

// colorize wraps msg with the given ANSI code, or returns msg unchanged when
// color is disabled.
func colorize(code, msg string) string {
	if noColor() {
		return msg
	}
	return code + msg + ansiReset
}

// banner prints the fastFSO startup header.
func banner() {
	if noColor() {
		fmt.Println("fastFSO dev")
		return
	}
	fmt.Println(ansiBold + ansiCyan + "🚀 fastFSO" + ansiReset + ansiDim + " dev" + ansiReset)
}

// step prints an action step: ==> msg
func step(msg string) {
	if noColor() {
		fmt.Println("==> " + msg)
		return
	}
	fmt.Println(ansiCyan + "==> " + ansiReset + msg)
}

// success prints a green checkmark: ✓ msg
func success(msg string) {
	if noColor() {
		fmt.Println("  OK " + msg)
		return
	}
	fmt.Println(ansiGreen + "  ✓ " + ansiReset + msg)
}

// warn prints a yellow warning: ⚠ msg
func warn(msg string) {
	if noColor() {
		fmt.Println("  WARN " + msg)
		return
	}
	fmt.Println(ansiYellow + "  ⚠ " + ansiReset + msg)
}

// fail prints a red error to stderr: ✗ msg
func fail(msg string) {
	if noColor() {
		fmt.Fprintln(os.Stderr, "  FAIL "+msg)
		return
	}
	fmt.Fprintln(os.Stderr, ansiRed+"  ✗ "+ansiReset+msg)
}

// info prints a dim/gray informational message.
func info(msg string) {
	if noColor() {
		fmt.Println("  " + msg)
		return
	}
	fmt.Println(ansiDim + "  " + msg + ansiReset)
}
