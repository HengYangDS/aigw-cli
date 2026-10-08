// Package main starts the AIGW command-line application.
package main

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/native"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if handled, code := native.RunCredentialSubprocess(args, os.Stdin, stdout, stderr, secrets.Service); handled {
		return code
	}
	app, err := cli.NewDefault()
	if err != nil {
		renderer := presentation.New(stderr, false)
		if len(args) > 0 && args[0] == "credential" {
			presentation.RenderCredentialError(renderer, err)
		} else {
			presentation.RenderError(renderer, presentation.ProblemError(
				"Cannot initialize AIGW", "", "The command did not start.",
				"Check AIGW's environment and selected credential backend, then retry.", err,
			), false)
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
