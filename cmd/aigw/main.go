// Package main starts the AIGW command-line application.
package main

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
	"aigw-cli/internal/terminal"
	"aigw-cli/internal/upgrade"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// credentialRole lets only the installed program change a native item's owner.
// A versioned reader copy, an extracted release or a development build cannot
// tell which program its installation currently runs.
func credentialRole(executable string, err error) native.Role {
	if err == nil && upgrade.InstalledProgram(executable) {
		return native.InstalledProgram
	}
	return native.OtherProgram
}

func run(args []string, stdout, stderr io.Writer) int {
	role := func() native.Role { return credentialRole(os.Executable()) }
	if handled, code := native.RunCredentialSubprocess(args, os.Stdin, stdout, stderr, secret.Service, role); handled {
		return code
	}
	app, err := cli.NewDefault(clients.Registry())
	if err != nil {
		renderer := terminal.NewRenderer(stderr, false)
		if len(args) > 0 && args[0] == "credential" {
			terminal.RenderCredentialError(renderer, err)
		} else {
			terminal.RenderError(renderer, terminal.ProblemError(
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
