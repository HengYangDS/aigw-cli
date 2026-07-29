package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
)

func main() {
	os.Exit(run(os.Args[0], os.Args[1:], os.Stdout, os.Stderr))
}

func run(program string, args []string, stdout, stderr io.Writer) int {
	app, err := cli.NewDefault()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "aigw:", err)
		return 1
	}
	app.Out = stdout
	app.Err = stderr
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(program)), ".exe")
	if name == "claude" || name == "claude.cmd" {
		err = cli.RunClaude(app, args)
	} else {
		err = cli.Execute(app, args)
	}
	if err != nil {
		cli.RenderError(app, err)
		return 1
	}
	return 0
}
