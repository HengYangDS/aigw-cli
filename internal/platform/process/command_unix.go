//go:build !windows

package process

import (
	"context"
	"os/exec"
)

func commandContext(ctx context.Context, plan Plan) *exec.Cmd {
	return exec.CommandContext(ctx, plan.Executable, plan.Args...)
}
