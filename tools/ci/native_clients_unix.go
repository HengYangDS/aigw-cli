//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// supplyInUse refuses a supply that a live process still uses: a client, or a
// process that a client started, names a path inside the supply in its command
// line.
func supplyInUse(owned string, inventory outputRunner) error {
	users, err := supplyProcesses(owned, inventory)
	if err != nil {
		return fmt.Errorf("prove that no process uses the native client supply: %w", err)
	}
	if len(users) != 0 {
		return fmt.Errorf("native client supply is still in use by process %s; retained %s", strings.Join(users, ", "), owned)
	}
	return nil
}

// supplyProcesses returns the IDs of live processes whose command line names a
// path inside the supply. Only IDs are reported; command lines stay private.
func supplyProcesses(owned string, inventory outputRunner) ([]string, error) {
	output, err := inventory(command{Name: "ps", Args: []string{"-A", "-ww", "-o", "pid=", "-o", "args="}, Env: []string{"LC_ALL=C"}, Timeout: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	self := strconv.Itoa(os.Getpid())
	var users []string
	for line := range strings.Lines(string(output)) {
		pid, arguments, _ := strings.Cut(strings.TrimSpace(line), " ")
		if pid != self && strings.Contains(arguments, owned+string(filepath.Separator)) {
			users = append(users, pid)
		}
	}
	return users, nil
}
