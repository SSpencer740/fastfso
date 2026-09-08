package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

// checkDocker verifies that Docker is available and running.
func checkDocker() error {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not running (docker info failed): %w", err)
	}
	return nil
}

// containerState returns the container status: "running", "exited", or "" (not found).
func containerState() string {
	var buf bytes.Buffer
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", containerName)
	cmd.Stdout = &buf
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

// waitForPostgres polls the container up to 30 times at 1s intervals, verifying
// the user and database exist by running an actual query via psql. This avoids
// a race with the postgres image's init scripts — pg_isready returns true as soon
// as the temporary init server accepts connections, before POSTGRES_USER is created.
func waitForPostgres() error {
	fmt.Print(colorize(ansiCyan, "    waiting for postgres "))
	for range 30 {
		cmd := exec.Command("docker", "exec", containerName,
			"psql", "-U", cfg.User, "-d", cfg.Name, "-c", "SELECT 1")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err == nil {
			fmt.Println(colorize(ansiGreen, " ready"))
			return nil
		}
		fmt.Print(colorize(ansiCyan, "."))
		time.Sleep(1 * time.Second)
	}
	fmt.Println()
	return fmt.Errorf("postgres did not become ready after 30s")
}

// runMigrations runs database migrations via the backend's migrate CLI.
func runMigrations() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	step("Running migrations...")
	cmd := exec.Command("go", "run", ".", "migrate", "up")
	cmd.Dir = root + "/backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "DATABASE_URL="+cfg.URL())
	return runPassthrough(cmd)
}

func cmdUp() {
	if cfg.External {
		step("External database mode — skipping Docker")
		info(fmt.Sprintf("host: %s:%s  user: %s  db: %s", cfg.Host, cfg.Port, cfg.User, cfg.Name))
		fmt.Println()

		if err := runMigrations(); err != nil {
			fail(fmt.Sprintf("migration failed: %v", err))
			os.Exit(1)
		}

		fmt.Println()
		success(colorize(ansiGreen+ansiBold, "migrations complete"))
		return
	}

	step("Checking Docker...")
	if err := checkDocker(); err != nil {
		fail(err.Error())
		os.Exit(1)
	}
	success("Docker is running")

	state := containerState()
	switch state {
	case "running":
		info("postgres container is already running")
	case "exited":
		step("Starting existing postgres container...")
		if err := run("docker", "start", containerName); err != nil {
			fail(fmt.Sprintf("failed to start container: %v", err))
			os.Exit(1)
		}
		success("Container started")
	default:
		port, err := findAvailablePort(dbDefaultPort)
		if err != nil {
			fail(err.Error())
			os.Exit(1)
		}
		if port != dbDefaultPort {
			warn(fmt.Sprintf("port %d is in use, using port %d instead", dbDefaultPort, port))
		}
		step("Creating postgres container...")
		hostPort := strconv.Itoa(port)
		if err := run("docker", "run", "-d",
			"--name", containerName,
			"-v", volumeName+":/var/lib/postgresql/data",
			"-e", "POSTGRES_USER="+cfg.User,
			"-e", "POSTGRES_PASSWORD="+cfg.Password,
			"-e", "POSTGRES_DB="+cfg.Name,
			"-p", hostPort+":5432",
			postgresImage,
		); err != nil {
			fail(fmt.Sprintf("failed to create container: %v", err))
			os.Exit(1)
		}
		success("Container created")
	}

	if err := waitForPostgres(); err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	if err := runMigrations(); err != nil {
		fail(fmt.Sprintf("migration failed: %v", err))
		os.Exit(1)
	}

	fmt.Println()
	success(colorize(ansiGreen+ansiBold, "postgres is ready"))
	info(fmt.Sprintf("host: %s:%s  user: %s  db: %s", cfg.Host, cfg.Port, cfg.User, cfg.Name))
}

func cmdDown() {
	if cfg.External {
		info("external database — skipping container removal")
		return
	}

	step("Removing postgres container...")
	cmd := exec.Command("docker", "rm", "-f", containerName)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		info("container not found (already removed)")
		return
	}
	success("container removed")
	info("volume " + volumeName + " preserved")
}

func cmdPs() {
	if cfg.External {
		fmt.Printf("  %s  %s\n", colorize(ansiBold, "mode:"), "external")
		fmt.Printf("  %s  %s\n", colorize(ansiBold, "host:"), cfg.Host+":"+cfg.Port)
		fmt.Printf("  %s  %s\n", colorize(ansiBold, "user:"), cfg.User)
		fmt.Printf("  %s    %s\n", colorize(ansiBold, "db:"), cfg.Name)
		return
	}

	state := containerState()
	if state == "" {
		info("no postgres container found")
		return
	}

	var stateColor string
	switch state {
	case "running":
		stateColor = ansiGreen
	case "exited":
		stateColor = ansiYellow
	default:
		stateColor = ansiDim
	}

	port, _ := containerPort()

	fmt.Printf("  %s  %s\n", colorize(ansiBold, "name:"), containerName)
	fmt.Printf("  %s %s\n", colorize(ansiBold, "state:"), colorize(stateColor, state))
	if port != "" {
		fmt.Printf("  %s  %s\n", colorize(ansiBold, "port:"), "localhost:"+port)
	}
}

func cmdPsql() {
	extra := os.Args[2:]

	if cfg.External {
		step("Connecting to postgres...")
		args := append([]string{cfg.URL()}, extra...)
		cmd := exec.Command("psql", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := runPassthrough(cmd); err != nil {
			fail(fmt.Sprintf("psql failed: %v", err))
			os.Exit(1)
		}
		return
	}

	state := containerState()
	if state != "running" {
		fail("postgres container is not running (run ./dev.sh up first)")
		os.Exit(1)
	}

	step("Connecting to postgres...")
	dockerArgs := []string{"exec"}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		dockerArgs = append(dockerArgs, "-it")
	}
	dockerArgs = append(dockerArgs, containerName, "psql", "-U", cfg.User, "-d", cfg.Name)
	dockerArgs = append(dockerArgs, extra...)
	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail(fmt.Sprintf("psql failed: %v", err))
		os.Exit(1)
	}
}

func cmdReset() {
	if cfg.External {
		info("external database — skipping container and volume removal")
		return
	}

	cmdDown()
	step("Removing data volume...")
	cmd := exec.Command("docker", "volume", "rm", volumeName)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		info("volume not found (already removed)")
		return
	}
	success("volume " + volumeName + " removed")
}
