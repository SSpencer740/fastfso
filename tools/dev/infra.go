package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/term"
)

func cmdInfra() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	envsDir := filepath.Join(root, "infra", "environments")
	envs := listEnvironments(envsDir)

	if len(os.Args) < 3 {
		infraUsage(envs)
		os.Exit(1)
	}

	env := os.Args[2]
	if !contains(envs, env) {
		fail(fmt.Sprintf("unknown environment: %s (available: %s)", env, strings.Join(envs, ", ")))
		os.Exit(1)
	}

	if len(os.Args) < 4 {
		fail("missing action")
		fmt.Println()
		infraUsage(envs)
		os.Exit(1)
	}

	action := os.Args[3]
	validActions := []string{"plan", "deploy", "apply", "destroy"}
	if !contains(validActions, action) {
		fail(fmt.Sprintf("unknown action: %s (available: %s)", action, strings.Join(validActions, ", ")))
		os.Exit(1)
	}

	extra := os.Args[4:]
	dir := filepath.Join(envsDir, env)
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	step("Running terraform init...")
	init := exec.Command("terraform", "init")
	init.Dir = dir
	init.Stdin = os.Stdin
	init.Stdout = os.Stdout
	init.Stderr = os.Stderr
	if err := runPassthrough(init); err != nil {
		fail(fmt.Sprintf("terraform init failed: %v", err))
		os.Exit(1)
	}
	success("terraform init complete")
	fmt.Println()

	var tfCmd string
	switch action {
	case "plan":
		tfCmd = "plan"
	case "deploy", "apply":
		tfCmd = "apply"
	case "destroy":
		tfCmd = "destroy"
	}

	args := []string{tfCmd}
	if !isTTY {
		args = append(args, "-input=false")
	}
	args = append(args, extra...)

	step(fmt.Sprintf("Running terraform %s...", tfCmd))
	tf := exec.Command("terraform", args...)
	tf.Dir = dir
	tf.Stdin = os.Stdin
	tf.Stdout = os.Stdout
	tf.Stderr = os.Stderr

	if err := runPassthrough(tf); err != nil {
		fail(fmt.Sprintf("terraform %s failed: %v", tfCmd, err))
		os.Exit(1)
	}
	success(fmt.Sprintf("terraform %s complete", tfCmd))
}

func listEnvironments(envsDir string) []string {
	entries, err := os.ReadDir(envsDir)
	if err != nil {
		fail(fmt.Sprintf("cannot read infra/environments: %v", err))
		os.Exit(1)
	}
	var envs []string
	for _, e := range entries {
		if e.IsDir() {
			envs = append(envs, e.Name())
		}
	}
	sort.Strings(envs)
	return envs
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func infraUsage(envs []string) {
	fmt.Fprintf(os.Stderr, "%s\n\n", colorize(ansiBold, "usage: dev.sh infra <environment> <action> [terraform args...]"))
	fmt.Fprintf(os.Stderr, "%s %s\n", colorize(ansiBold, "environments:"), strings.Join(envs, ", "))
	fmt.Fprintf(os.Stderr, "%s    %s\n", colorize(ansiBold, "actions:"), "plan, deploy (or apply), destroy")
}
