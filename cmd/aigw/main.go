package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
)

func main() {
	app, err := cli.NewDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, "aigw:", err)
		os.Exit(1)
	}
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
	if name == "claude" {
		err = cli.RunClaude(app, os.Args[1:])
	} else {
		err = cli.NewRoot(app).Execute()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "aigw:", err)
		os.Exit(1)
	}
}
