//go:build !windows

package process

import "os/exec"

func startCapturedProcess(cmd *exec.Cmd) (func() error, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() error { return nil }, nil
}
