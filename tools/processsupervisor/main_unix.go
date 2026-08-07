//go:build !windows

package main

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func startOwned(command *exec.Cmd) (func(), error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return nil, err
	}
	cleaned := false
	return func() {
		if cleaned || command.Process == nil {
			return
		}
		cleaned = true
		pid := command.Process.Pid
		if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
			time.Sleep(time.Second)
		}
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			_ = command.Process.Kill()
		}
	}, nil
}
