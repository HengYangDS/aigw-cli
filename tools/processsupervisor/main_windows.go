//go:build windows

package main

import (
	"os/exec"

	"aigw-cli/internal/winjob"
)

func startOwned(command *exec.Cmd) (func(), error) {
	return winjob.Start(command)
}
