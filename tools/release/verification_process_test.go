package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"aigw-cli/internal/platform"
)

type verificationResourceProcess struct {
	PID     int    `json:"pid"`
	Home    string `json:"home"`
	Control string `json:"control"`
	Role    string `json:"role"`
}

func runVerificationResourceRole(role string) int {
	if role != "child" && role != "unrelated" {
		return 2
	}
	if err := writeVerificationProcess(filepath.Join(os.Getenv("AIGW_TEST_RESOURCE_CONTROL"), role+".json")); err != nil {
		return 2
	}
	return holdVerificationProcess(role, 3*time.Minute)
}

// runVerificationResourceClient publishes an owned parent and child under the
// client's disposable home, then ends as the selected resource case requires.
func runVerificationResourceClient(home string, answer func() int) int {
	control := os.Getenv("AIGW_TEST_RESOURCE_CONTROL")
	if err := os.Setenv("AIGW_TEST_RESOURCE_HOME", home); err != nil {
		return 2
	}
	child := exec.Command(os.Args[0])
	child.Env = environmentWith(os.Environ(), map[string]string{"AIGW_TEST_RESOURCE_ROLE": "child"})
	if err := child.Start(); err != nil {
		return 2
	}
	if err := awaitVerificationFile(filepath.Join(control, "child.json")); err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		return 2
	}
	if err := writeVerificationProcess(filepath.Join(control, "parent.json")); err != nil {
		return 2
	}
	if err := awaitVerificationFile(filepath.Join(control, "release")); err != nil {
		return 2
	}
	switch os.Getenv("AIGW_TEST_RESOURCE_CASE") {
	case "success":
		return answer()
	case "failure":
		return 17
	case "parent-exit":
		return 0
	case "interrupt", "deadline":
		return holdVerificationProcess("parent", 2*time.Minute)
	default:
		return 2
	}
}

func writeVerificationProcess(path string) error {
	role := os.Getenv("AIGW_TEST_RESOURCE_ROLE")
	if role == "" {
		role = "parent"
	}
	data, err := json.Marshal(verificationResourceProcess{
		PID: os.Getpid(), Home: os.Getenv("AIGW_TEST_RESOURCE_HOME"),
		Control: os.Getenv("AIGW_TEST_RESOURCE_CONTROL"), Role: role,
	})
	if err != nil {
		return err
	}
	return platform.WriteFileAtomicExactMode(path, data, 0o600)
}

func TestVerificationProcessPublicationDoesNotMutateObservedBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "parent.json")
	if err := writeVerificationProcess(path); err != nil {
		t.Fatal(err)
	}
	observed := filepath.Join(root, "observed.json")
	if err := os.Link(path, observed); err != nil {
		t.Fatal(err)
	}
	original := readFile(t, observed)
	t.Setenv("AIGW_TEST_RESOURCE_ROLE", "child")
	if err := writeVerificationProcess(path); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, observed), original) {
		t.Fatal("publishing process readiness changed bytes already observed by a reader")
	}
	var process verificationResourceProcess
	if err := json.Unmarshal(readFile(t, path), &process); err != nil || process.Role != "child" {
		t.Fatalf("published process = %#v, error = %v", process, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("publication left temporary entries: %v, %v", entries, err)
	}
}

func holdVerificationProcess(role string, limit time.Duration) int {
	stop := filepath.Join(os.Getenv("AIGW_TEST_RESOURCE_CONTROL"), role+".stop")
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stop); err == nil {
			return 0
		} else if !errors.Is(err, os.ErrNotExist) {
			return 2
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 2
}

func awaitVerificationFile(path string) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("verification fixture did not reach %s", filepath.Base(path))
}

func observeVerificationProcess(t *testing.T, pid int, control, role string) func() bool {
	t.Helper()
	var owned verificationResourceProcess
	if err := json.Unmarshal(readFile(t, filepath.Join(control, role+".json")), &owned); err != nil ||
		owned.PID != pid || owned.Control != control || owned.Role != role {
		t.Fatalf("owned process identity does not match its private control channel: %v", err)
	}
	alive := func() bool {
		if runtime.GOOS == "linux" {
			data, err := os.ReadFile(filepath.Join(string(filepath.Separator), "proc", strconv.Itoa(pid), "stat"))
			if verificationProcessAbsent(err) {
				return false
			}
			if err != nil {
				t.Fatalf("observe owned process %d: %v", pid, err)
			}
			fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
			if len(fields) > 0 && (fields[0] == "Z" || fields[0] == "X") {
				return false
			}
		}
		err := syscall.Kill(pid, 0)
		if verificationProcessAbsent(err) {
			return false
		}
		if err != nil {
			t.Fatalf("observe owned process %d: %v", pid, err)
		}
		return true
	}
	if role == "unrelated" {
		return alive
	}
	t.Cleanup(func() {
		if !alive() {
			return
		}
		if err := os.WriteFile(filepath.Join(control, role+".stop"), []byte("stop"), 0o600); err != nil {
			t.Errorf("stop exact owned fixture %s: %v", role, err)
			return
		}
		deadline := time.Now().Add(5 * time.Second)
		for alive() && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if alive() {
			t.Errorf("owned fixture %s did not stop after assertion failure", role)
		}
	})
	return alive
}

func verificationProcessAbsent(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}

func TestVerificationProcessAbsenceClassifiesKernelExitRace(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		absent bool
	}{
		{"running", nil, false},
		{"removed", &os.PathError{Op: "open", Path: "/proc/123/stat", Err: syscall.ENOENT}, true},
		{"exited during read", &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.ESRCH}, true},
		{"signal target exited", syscall.ESRCH, true},
		{"permission denied", &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.EACCES}, false},
		{"operation denied", syscall.EPERM, false},
		{"read failed", &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.EIO}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := verificationProcessAbsent(test.err); got != test.absent {
				t.Errorf("process absence for %v = %t, want %t", test.err, got, test.absent)
			}
		})
	}
}
