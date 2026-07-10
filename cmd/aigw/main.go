package main

import (
	"fmt"
	"os"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
)

func main() {
	app, err := cli.NewDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, "aigw:", err)
		os.Exit(1)
	}
	if err := cli.NewRoot(app).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "aigw:", err)
		os.Exit(1)
	}
}
