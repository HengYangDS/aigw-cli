//go:build !windows

package process

import (
	"errors"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func startCapturedProcess(cmd *exec.Cmd) (func() error, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() error { return terminateCapturedProcessGroup(cmd.Process.Pid, unix.Kill) }, nil
}

func terminateCapturedProcessGroup(pid int, signal func(int, syscall.Signal) error) error {
	err := signal(-pid, unix.SIGKILL)
	if errors.Is(err, unix.ESRCH) {
		return nil
	}
	if !errors.Is(err, unix.EPERM) {
		return err
	}
	// A retiring native group can briefly reject a signal after its leader
	// exits. Accept only observed disappearance; a live denied group fails.
	deadline := time.Now().Add(capturedProcessWaitDelay)
	for time.Now().Before(deadline) {
		observed := signal(-pid, 0)
		if errors.Is(observed, unix.ESRCH) {
			return nil
		}
		if !errors.Is(observed, unix.EPERM) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}
