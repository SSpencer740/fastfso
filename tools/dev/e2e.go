package main

import (
	"os"
	"os/exec"
)

func cmdE2E() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	step("Running Playwright end-to-end tests...")
	cmd := exec.Command("npx", "playwright", "test")
	cmd.Dir = root + "/e2e"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("e2e tests failed")
		os.Exit(1)
	}
	success("e2e tests passed")
}

func cmdE2EUI() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	step("Opening Playwright UI mode...")
	cmd := exec.Command("npx", "playwright", "test", "--ui")
	cmd.Dir = root + "/e2e"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("e2e UI exited with error")
		os.Exit(1)
	}
}
