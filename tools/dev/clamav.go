package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	clamavContainerName = "fastfso-clamav"
	clamavImage         = "clamav/clamav:stable"
	clamavDefaultPort   = 3310
)

// clamavContainerState returns the container status: "running", "exited", or "" (not found).
func clamavContainerState() string {
	var buf bytes.Buffer
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", clamavContainerName)
	cmd.Stdout = &buf
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

// clamavHostPort reads the host port mapped to 3310/tcp from the container config.
func clamavHostPort() (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command("docker", "inspect", "--format",
		`{{(index (index .HostConfig.PortBindings "3310/tcp") 0).HostPort}}`,
		clamavContainerName)
	cmd.Stdout = &buf
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not read clamav port: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

// waitForClamAV polls clamd's PING command until the signature database has loaded.
// First-time startup downloads ~200MB of signatures and can take several minutes.
func waitForClamAV() error {
	fmt.Print(colorize(ansiCyan, "    waiting for clamd (signature load can take a few minutes on first start) "))
	for range 360 { // up to 6 minutes
		cmd := exec.Command("docker", "exec", clamavContainerName,
			"sh", "-c", `echo "PING" | nc -w 2 127.0.0.1 3310`)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = nil
		if err := cmd.Run(); err == nil && strings.Contains(out.String(), "PONG") {
			fmt.Println(colorize(ansiGreen, " ready"))
			return nil
		}
		fmt.Print(colorize(ansiCyan, "."))
		time.Sleep(1 * time.Second)
	}
	fmt.Println()
	return fmt.Errorf("clamd did not become ready within 6 minutes")
}

func cmdClamAVUp() {
	step("Checking Docker...")
	if err := checkDocker(); err != nil {
		fail(err.Error())
		os.Exit(1)
	}
	success("Docker is running")

	state := clamavContainerState()
	switch state {
	case "running":
		info("clamav container is already running")
	case "exited":
		step("Starting existing clamav container...")
		if err := run("docker", "start", clamavContainerName); err != nil {
			fail(fmt.Sprintf("failed to start container: %v", err))
			os.Exit(1)
		}
		success("Container started")
	default:
		port, err := findAvailablePort(clamavDefaultPort)
		if err != nil {
			fail(err.Error())
			os.Exit(1)
		}
		if port != clamavDefaultPort {
			warn(fmt.Sprintf("port %d is in use, using port %d instead", clamavDefaultPort, port))
		}
		step("Creating clamav container...")
		hostPort := strconv.Itoa(port)
		if err := run("docker", "run", "-d",
			"--name", clamavContainerName,
			"-p", hostPort+":3310",
			clamavImage,
		); err != nil {
			fail(fmt.Sprintf("failed to create container: %v", err))
			os.Exit(1)
		}
		success("Container created")
	}

	if err := waitForClamAV(); err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	port, _ := clamavHostPort()
	fmt.Println()
	success(colorize(ansiGreen+ansiBold, "clamav is ready"))
	if port != "" {
		info("addr: localhost:" + port)
	}
	info("./dev.sh serve will auto-detect the running container and set CLAMAV_ADDR")
}

func cmdClamAVDown() {
	step("Removing clamav container...")
	cmd := exec.Command("docker", "rm", "-f", clamavContainerName)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		info("container not found (already removed)")
		return
	}
	success("container removed")
}

func cmdClamAVPs() {
	state := clamavContainerState()
	if state == "" {
		info("no clamav container found")
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

	port, _ := clamavHostPort()

	fmt.Printf("  %s  %s\n", colorize(ansiBold, "name:"), clamavContainerName)
	fmt.Printf("  %s %s\n", colorize(ansiBold, "state:"), colorize(stateColor, state))
	if port != "" {
		fmt.Printf("  %s  %s\n", colorize(ansiBold, "port:"), "localhost:"+port)
	}
}

func cmdClamAV() {
	if len(os.Args) < 3 {
		fail("usage: dev.sh clamav <up|down|ps>")
		os.Exit(1)
	}
	switch os.Args[2] {
	case "up":
		cmdClamAVUp()
	case "down":
		cmdClamAVDown()
	case "ps":
		cmdClamAVPs()
	default:
		fail(fmt.Sprintf("unknown clamav subcommand: %s", os.Args[2]))
		os.Exit(1)
	}
}
