package main

import (
	"fmt"
	"os"
)

func main() {
	banner()
	fmt.Println()

	cfg = loadDBConfig()

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "up":
		cmdUp()
	case "down":
		cmdDown()
	case "serve":
		cmdServe()
	case "ps":
		cmdPs()
	case "reset":
		cmdReset()
	case "test":
		cmdTest()
	case "test-integration":
		cmdTestIntegration()
	case "fmt":
		cmdFmt()
	case "lint":
		cmdLint()
	case "psql":
		cmdPsql()
	case "seed":
		cmdSeed()
	case "update":
		cmdUpdate()
	case "infra":
		cmdInfra()
	case "registry":
		if len(os.Args) > 2 && os.Args[2] == "prune" {
			cmdRegistryPrune()
		} else {
			cmdRegistry()
		}
	case "loc":
		cmdLoc()
	case "e2e":
		cmdE2E()
	case "e2e-ui":
		cmdE2EUI()
	case "clamav":
		cmdClamAV()
	default:
		fail(fmt.Sprintf("unknown command: %s", os.Args[1]))
		fmt.Println()
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "%s\n\n", colorize(ansiBold, "usage: dev.sh <command>"))
	fmt.Fprintln(os.Stderr, colorize(ansiBold, "commands:"))
	cmds := []struct{ name, desc string }{
		{"up", "create/start postgres container and run migrations"},
		{"down", "stop and remove postgres container (keeps data volume)"},
		{"serve", "start backend and frontend dev servers"},
		{"ps", "show postgres container status"},
		{"reset", "remove postgres container and data volume"},
		{"test", "run backend unit tests"},
		{"test-integration", "run backend integration tests against PostgreSQL"},
		{"fmt", "format Go, Terraform, and frontend code"},
		{"lint", "run backend and frontend linters"},
		{"psql", "open a psql shell on the database"},
		{"seed [env]", "seed admin account (local default, or cloud env)"},
		{"update", "update Go and frontend dependencies"},
		{"infra", "run terraform plan/deploy/destroy for an environment"},
		{"registry", "show Artifact Registry image stats"},
		{"registry prune", "delete images older than 30 days"},
		{"loc", "lines of code by language (requires cloc)"},
		{"e2e", "run Playwright end-to-end tests"},
		{"e2e-ui", "run Playwright tests in interactive UI mode"},
		{"clamav <up|down|ps>", "manage local ClamAV container for malware scan testing"},
	}
	for _, c := range cmds {
		fmt.Fprintf(os.Stderr, "  %-18s  %s\n",
			colorize(ansiCyan+ansiBold, c.name),
			colorize(ansiDim, c.desc))
	}
}
