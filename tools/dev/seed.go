package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func cmdSeed() {
	args := os.Args[2:]

	// Cloud environment path
	if len(args) > 0 {
		cloudSeed(args[0])
		return
	}

	// Local path
	if !cfg.External {
		state := containerState()
		if state != "running" {
			fail("postgres is not running (run ./dev.sh up first)")
			os.Exit(1)
		}
	}

	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	step("Seeding database...")
	cmd := exec.Command("go", "run", ".", "seed")
	cmd.Dir = root + "/backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "DATABASE_URL="+cfg.URL())
	if err := runPassthrough(cmd); err != nil {
		fail(fmt.Sprintf("seed failed: %v", err))
		os.Exit(1)
	}

	fmt.Println()
	success("seed account created")
	info("email:    admin@example.com")
	info("password: admin")
	info("role:     super admin")
}

func cloudSeed(env string) {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	envsDir := filepath.Join(root, "infra", "environments")
	envs := listEnvironments(envsDir)

	if !contains(envs, env) {
		fail(fmt.Sprintf("unknown environment: %s (available: %s)", env, strings.Join(envs, ", ")))
		os.Exit(1)
	}

	tfvarsPath := filepath.Join(envsDir, env, "terraform.tfvars")
	vars, err := parseTFVars(tfvarsPath)
	if err != nil {
		fail(fmt.Sprintf("failed to parse tfvars: %v", err))
		os.Exit(1)
	}

	projectID := vars["project_id"]
	region := vars["region"]
	if projectID == "" || region == "" {
		fail("terraform.tfvars must contain project_id and region")
		os.Exit(1)
	}

	jobName := fmt.Sprintf("fastfso-%s-seed", env)

	step(fmt.Sprintf("Executing Cloud Run seed job %s...", jobName))
	cmd := exec.Command("gcloud", "run", "jobs", "execute", jobName,
		"--project", projectID,
		"--region", region,
		"--wait",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail(fmt.Sprintf("cloud seed failed: %v", err))
		os.Exit(1)
	}

	fmt.Println()
	success(fmt.Sprintf("seed job executed in %s", env))
}

// parseTFVars reads a terraform.tfvars file and returns key-value pairs.
func parseTFVars(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, "\"")
		vars[key] = val
	}
	return vars, scanner.Err()
}
