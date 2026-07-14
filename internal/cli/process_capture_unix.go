//go:build !windows

package cli

import "os/exec"

func startCapturedProcess(cmd *exec.Cmd) (func(), error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() {}, nil
}
