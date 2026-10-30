package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func runVerificationResourceRole(role string, args []string) int {
	if role == "interrupt" {
		if len(args) != 1 {
			return 2
		}
		pid, err := strconv.Atoi(args[0])
		if err != nil {
			return 2
		}
		if err := sendVerificationConsoleInterrupt(pid); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}
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
	prepareVerificationInterrupt(child)
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
