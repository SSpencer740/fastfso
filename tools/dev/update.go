package main

import (
	"fmt"
	"os"
	"os/exec"
)

func cmdUpdate() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	backendDir := root + "/backend"

	step("Updating Go dependencies...")
	cmd := exec.Command("go", "get", "-u", "./...")
	cmd.Dir = backendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("go get -u failed")
		os.Exit(1)
	}
	success("dependencies updated")

	fmt.Println()
	step("Tidying go.mod...")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = backendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("go mod tidy failed")
		os.Exit(1)
	}
	success("go.mod tidied")

	fmt.Println()
	frontendDir := root + "/frontend"

	step("Updating frontend dependencies...")
	cmd = exec.Command("npm", "update")
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runPassthrough(cmd); err != nil {
		fail("npm update failed")
		os.Exit(1)
	}
	success("frontend dependencies updated")
}
