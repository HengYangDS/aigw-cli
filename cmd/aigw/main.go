package main

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/presentation"
	"fmt"
	"io"
	"os"
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
	err = cli.Execute(app, args)
	if err != nil {
		presentation.RenderError(app.Renderer(), err)
		return 1
	}
	return 0
}
