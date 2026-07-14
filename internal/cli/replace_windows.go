//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
)

func replaceProcess(plan adapters.ProcessPlan) error {
	cmd := exec.Command(plan.Executable, plan.Args...)
	cmd.Env = plan.Env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to run %s: %w", plan.Executable, err)
	}
	return nil
}
