//go:build !windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A job cancellation signals only the supplier. Its running installer, and the
// process that installer started, must stop without cooperating, and the
// partial supply must be removed before the supplier fails.
func TestNativeClientSupplyStopsItsBlockingInstallerOnSignal(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, tools := t.TempDir(), t.TempDir()
	for _, name := range []string{"ci", "mise", "npm"} {
		if err := os.Symlink(self, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	record, githubEnvironment := filepath.Join(t.TempDir(), "installers"), filepath.Join(root, "github-env")
	release := fixtureReleaseFor(runtime.GOOS + "/" + runtime.GOARCH)
	arguments := []string{"native-clients", "supply", "--root", root, "--github-env", githubEnvironment, "--codex", release.codex, "--claude", release.claude}
	if release.hermesCommit != "" {
		arguments = append(arguments, "--hermes-commit", release.hermesCommit, "--hermes-installer-sha256", release.hermesInstaller)
	}
	output, err := os.Create(filepath.Join(t.TempDir(), "supplier.out"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = output.Close() }()
	supplier := exec.Command(filepath.Join(tools, "ci"), arguments...)
	supplier.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), testInstallerRecord+"="+record)
	supplier.Stdout, supplier.Stderr = output, output
	if err := supplier.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- supplier.Wait() }()
	installers := recordedInstallers(t, record, exited)
	t.Cleanup(func() {
		for _, pid := range installers {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	if err := supplier.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// The bound observes the supplier; it is not a product deadline.
	select {
	case err = <-exited:
	case <-time.After(30 * time.Second):
		_ = supplier.Process.Kill()
		t.Fatalf("supplier still ran 30 s after SIGTERM (%v): %s", <-exited, readFile(t, output.Name()))
	}
	if exit, failed := errors.AsType[*exec.ExitError](err); !failed || exit.ExitCode() != 1 {
		t.Fatalf("interrupted supplier = %v: %s", err, readFile(t, output.Name()))
	}
	if diagnostics := string(readFile(t, output.Name())); !strings.Contains(diagnostics, "native client supply was interrupted") {
		t.Fatalf("interrupted supplier diagnostics: %s", diagnostics)
	}
	for _, pid := range installers {
		for deadline := time.Now().Add(10 * time.Second); !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH); time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("installer process %d outlived its interrupted supply", pid)
			}
		}
	}
	if left, _ := filepath.Glob(filepath.Join(root, clientSupplyPrefix+"*")); len(left) != 0 {
		t.Fatalf("interrupted supply survived: %q", left)
	}
	if lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n"); len(lines) != 1 {
		t.Fatalf("interrupted supply exported %q", lines)
	}
}

// recordedInstallers waits until both fake installers have recorded their
// process IDs, and fails if the supplier exits first.
func recordedInstallers(t *testing.T, record string, exited <-chan error) []int {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		data, err := os.ReadFile(record)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if fields := strings.Fields(string(data)); len(fields) == 2 {
			var pids []int
			for _, field := range fields {
				pid, err := strconv.Atoi(field)
				if err != nil {
					t.Fatal(err)
				}
				pids = append(pids, pid)
			}
			return pids
		}
		select {
		case err := <-exited:
			t.Fatalf("supplier exited before its installers started: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake installers did not start: %q", data)
		}
	}
}

func TestNativeClientRetirementPreservesASupplyInUse(t *testing.T) {
	owned, err := os.MkdirTemp(t.TempDir(), clientSupplyPrefix)
	if err != nil {
		t.Fatal(err)
	}
	// A real process names the supply in its command line, as a client or a
	// Hermes gateway started from it does.
	process := exec.Command("/bin/sleep", "30")
	process.Args[0] = filepath.Join(owned, "home", ".local", "bin", "hermes")
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = process.Process.Kill()
		_ = process.Wait()
	})
	pid := strconv.Itoa(process.Process.Pid)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		users, err := supplyProcesses(owned, systemOutputRunner)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(users, pid) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("process inventory never named the supply's user %s: %q", pid, users)
		}
	}
	if err := retireClients(owned, systemOutputRunner); err == nil || !strings.Contains(err.Error(), "still in use by process "+pid) {
		t.Fatalf("retirement of a supply in use = %v", err)
	}
	if _, err := os.Stat(owned); err != nil {
		t.Fatalf("retirement removed a supply in use: %v", err)
	}
	if err := process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = process.Wait()
	unproved := errors.New("ps exited with status 1")
	if err := retireClients(owned, func(command) ([]byte, error) { return nil, unproved }); !errors.Is(err, unproved) {
		t.Fatalf("retirement with an unproved inventory = %v", err)
	}
	if err := retireClients(owned, systemOutputRunner); err != nil {
		t.Fatalf("retirement after the process exited = %v", err)
	}
	if _, err := os.Lstat(owned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired supply survived: %v", err)
	}
}
