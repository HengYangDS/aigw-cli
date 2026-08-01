//go:build !windows

package process

import (
	"fmt"
	"os/exec"
	"syscall"
)

// unixExec is the process-image replacement primitive. Tests substitute it to
// exercise failure and success paths without terminating the test process.
var unixExec = syscall.Exec

func replaceProcess(plan Plan) error {
	executable, err := exec.LookPath(plan.Executable)
	if err != nil {
		return fmt.Errorf("Failed to resolve %s: %w", plan.Executable, err)
	}
	argv := append([]string{executable}, plan.Args...)
	if err := unixExec(executable, argv, plan.Env); err != nil {
		return fmt.Errorf("Failed to replace AIGW with %s: %w", executable, err)
	}
	return nil
}
