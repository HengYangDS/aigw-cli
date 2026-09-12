//go:build windows

package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func commandContext(ctx context.Context, plan Plan) *exec.Cmd {
	extension := strings.ToLower(filepath.Ext(plan.Executable))
	if extension != ".cmd" && extension != ".bat" {
		return exec.CommandContext(ctx, plan.Executable, plan.Args...)
	}
	interpreter := os.Getenv("ComSpec")
	if interpreter == "" {
		interpreter = "cmd.exe"
	}
	var commandLine strings.Builder
	commandLine.WriteString(`"` + strings.ReplaceAll(plan.Executable, `"`, `""`) + `"`)
	for _, argument := range plan.Args {
		commandLine.WriteByte(' ')
		commandLine.WriteString(windows.EscapeArg(argument))
	}
	cmd := exec.CommandContext(ctx, interpreter)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `/d /s /c "` + commandLine.String() + `"`}
	return cmd
}
