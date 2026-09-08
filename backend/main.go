package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "serve":
		cmdServe()
	case "migrate":
		cmdMigrate()
	case "seed":
		cmdSeed()
	default:
		fmt.Fprintf(os.Stderr, "usage: fastfso <serve|migrate|seed>\n")
		os.Exit(1)
	}
}
