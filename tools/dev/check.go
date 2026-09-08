package main

import (
	"fmt"
	"os"
	"os/exec"
)

func cmdTest() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	failed := false

	step("Running backend tests...")
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root + "/backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("backend tests failed")
		failed = true
	} else {
		success("backend tests passed")
	}

	fmt.Println()
	info("no frontend tests configured")

	if failed {
		os.Exit(1)
	}
}

func cmdTestIntegration() {
	// Ensure postgres is running and migrations are current.
	cmdUp()
	cfg = loadDBConfig() // refresh port in case cmdUp created a new container
	fmt.Println()

	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	step("Running integration tests...")
	cmd := exec.Command("go", "test", "-tags", "integration", "-p", "1", "-v", "-count=1", "./...")
	cmd.Dir = root + "/backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "INTEGRATION_DATABASE_URL="+cfg.WithName("fastfso_test").URL())
	if err := runPassthrough(cmd); err != nil {
		fail("integration tests failed")
		os.Exit(1)
	}
	success("integration tests passed")
}

func cmdFmt() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	failed := false

	// Go: gofmt
	step("Formatting Go (gofmt)...")
	cmd := exec.Command("gofmt", "-w", ".")
	cmd.Dir = root + "/backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("gofmt failed")
		failed = true
	} else {
		success("gofmt done")
	}

	// Go: goimports
	fmt.Println()
	step("Formatting Go (goimports)...")
	if _, lookErr := exec.LookPath("goimports"); lookErr != nil {
		warn("goimports not found — install with: go install golang.org/x/tools/cmd/goimports@latest")
	} else {
		cmd = exec.Command("goimports", "-w", ".")
		cmd.Dir = root + "/backend"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := runPassthrough(cmd); err != nil {
			fail("goimports failed")
			failed = true
		} else {
			success("goimports done")
		}
	}

	// Terraform
	fmt.Println()
	step("Formatting Terraform...")
	if _, lookErr := exec.LookPath("terraform"); lookErr != nil {
		warn("terraform not found — skipping")
	} else {
		cmd = exec.Command("terraform", "fmt", "-recursive")
		cmd.Dir = root + "/infra"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := runPassthrough(cmd); err != nil {
			fail("terraform fmt failed")
			failed = true
		} else {
			success("terraform fmt done")
		}
	}

	// Frontend: eslint --fix
	fmt.Println()
	step("Formatting frontend (eslint --fix)...")
	cmd = exec.Command("npx", "eslint", "--fix", ".")
	cmd.Dir = root + "/frontend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("frontend eslint --fix failed")
		failed = true
	} else {
		success("frontend eslint --fix done")
	}

	if failed {
		os.Exit(1)
	}
}

func cmdLint() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	failed := false

	step("Linting backend...")
	cmd := exec.Command("golangci-lint", "run")
	cmd.Dir = root + "/backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("backend lint failed")
		failed = true
	} else {
		success("backend lint passed")
	}

	fmt.Println()
	step("Linting frontend...")
	cmd = exec.Command("npx", "eslint", ".")
	cmd.Dir = root + "/frontend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("frontend lint failed")
		failed = true
	} else {
		success("frontend lint passed")
	}

	if failed {
		os.Exit(1)
	}
}
