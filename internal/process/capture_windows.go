//go:build windows

package process

import (
	"fmt"
	"os/exec"

	"aigw-cli/internal/winjob"
)

// startCapturedProcess puts an AIGW-owned non-interactive child into a Job
// Object. Closing the Job after Wait returns terminates only this invocation's
// remaining descendants, including wrappers that inherited stdout/stderr.
func startCapturedProcess(cmd *exec.Cmd) (func(), error) {
	cleanup, err := winjob.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start captured child: %w", err)
	}
	return cleanup, nil
}
