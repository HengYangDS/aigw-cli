// Package main starts the AIGW command-line application.
package main

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/native"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if handled, code := native.RunCredentialSubprocess(args, os.Stdin, stdout, secrets.Service); handled {
		return code
	}
	app, err := cli.NewDefault()
	if err != nil {
		if len(args) > 0 && args[0] == "credential" {
			presentation.RenderCredentialError(presentation.New(stderr, false), err)
		} else {
			_, _ = fmt.Fprintln(stderr, "aigw:", err)
		}
		return 1
	}
	app.Out = stdout
	app.Err = stderr
	err = cli.Execute(app, args)
	if err != nil {
		return 1
	}
	return 0
}
