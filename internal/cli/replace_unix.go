//go:build !windows

package cli

import (
	"fmt"
	"os/exec"
	"syscall"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
)

func replaceProcess(plan adapters.ProcessPlan) error {
	executable, err := exec.LookPath(plan.Executable)
	if err != nil {
		return fmt.Errorf("解析 %s 失败：%w", plan.Executable, err)
	}
	argv := append([]string{executable}, plan.Args...)
	if err := syscall.Exec(executable, argv, plan.Env); err != nil {
		return fmt.Errorf("用 %s 替换 AIGW 失败：%w", executable, err)
	}
	return nil
}
