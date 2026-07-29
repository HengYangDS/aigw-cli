//go:build windows

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
)

const (
	windowsPipeDrainRoleEnvironment       = "AIGW_TEST_WINDOWS_PIPE_DRAIN_ROLE"
	windowsPipeDrainChildReadyEnvironment = "AIGW_TEST_WINDOWS_PIPE_DRAIN_CHILD_READY"
	windowsPipeDrainReleaseEnvironment    = "AIGW_TEST_WINDOWS_PIPE_DRAIN_RELEASE"
	windowsPipeDrainBarrierEnvironment    = "AIGW_TEST_WINDOWS_PIPE_DRAIN_BARRIER"
	windowsPipeDrainPIDEnvironment        = "AIGW_TEST_WINDOWS_PIPE_DRAIN_PID"
)

func TestMain(m *testing.M) {
	if os.Getenv(windowsPipeDrainRoleEnvironment) == "oversized-output" {
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), capturedProcessOutputLimit+1))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type windowsPipeDrainFixture struct {
	plan       adapters.ProcessPlan
	childReady string
	release    string
	barrier    string
	descendant string
}

func TestRunCaptureBoundsPipeDrainAfterChildExit(t *testing.T) {
	if runWindowsPipeDrainHelper(t, "TestRunCaptureBoundsPipeDrainAfterChildExit") {
		return
	}
	fixture := newWindowsPipeDrainFixture(t, "TestRunCaptureBoundsPipeDrainAfterChildExit", "direct-exit")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := (ProcessRunner{}).RunCapture(ctx, fixture.plan)
		result <- err
	}()

	if err := releaseWindowsPipeDrainChild(fixture); err != nil {
		cancel()
		t.Fatal(err)
	}
	started := time.Now()
	var runErr error
	select {
	case runErr = <-result:
	case <-time.After(capturedProcessWaitDelay + 5*time.Second):
		cancel()
		t.Fatal("RunCapture did not return after the bounded Windows pipe-drain delay")
	}
	if !errors.Is(runErr, exec.ErrWaitDelay) {
		t.Fatalf("RunCapture error = %v, want exec.ErrWaitDelay", runErr)
	}
	if elapsed := time.Since(started); elapsed > capturedProcessWaitDelay+3*time.Second {
		t.Fatalf("RunCapture took %s after child release, expected bounded pipe drain", elapsed)
	}
}

func TestRunCaptureReportsPipeDrainAfterContextDeadline(t *testing.T) {
	if runWindowsPipeDrainHelper(t, "TestRunCaptureReportsPipeDrainAfterContextDeadline") {
		return
	}
	fixture := newWindowsPipeDrainFixture(t, "TestRunCaptureReportsPipeDrainAfterContextDeadline", "direct-deadline")
	ctx := newDeadlineSignalContext()
	result := make(chan error, 1)
	go func() {
		_, err := (ProcessRunner{}).RunCapture(ctx, fixture.plan)
		result <- err
	}()

	if err := releaseWindowsPipeDrainChild(fixture); err != nil {
		ctx.expire()
		t.Fatal(err)
	}
	if err := waitForFixtureFile(fixture.barrier, 5*time.Second); err != nil {
		ctx.expire()
		t.Fatalf("Windows pipe-drain fixture did not reach the deadline barrier: %v", err)
	}
	ctx.expire()
	var runErr error
	select {
	case runErr = <-result:
	case <-time.After(capturedProcessWaitDelay + 5*time.Second):
		t.Fatal("RunCapture did not return after the bounded Windows pipe-drain delay")
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "exceeded its verification limit and its output pipes did not close within") {
		t.Fatalf("RunCapture error = %v, want Windows pipe-drain deadline diagnostic", runErr)
	}
}

func TestRunCaptureRejectsOversizedWindowsOutput(t *testing.T) {
	if runWindowsPipeDrainHelper(t, "TestRunCaptureRejectsOversizedWindowsOutput") {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_, err = (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: executable,
		Args:       []string{"-test.run=^TestRunCaptureRejectsOversizedWindowsOutput$"},
		Env:        append(os.Environ(), windowsPipeDrainRoleEnvironment+"=oversized-output"),
	})
	want := fmt.Sprintf("captured output from %s exceeds %d bytes", executable, capturedProcessOutputLimit)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("RunCapture error = %v, want %q", err, want)
	}
}

func newWindowsPipeDrainFixture(t *testing.T, testName, role string) windowsPipeDrainFixture {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fixture := windowsPipeDrainFixture{
		childReady: filepath.Join(dir, "child-ready"),
		release:    filepath.Join(dir, "release"),
		barrier:    filepath.Join(dir, "barrier"),
		descendant: filepath.Join(dir, "descendant.pid"),
	}
	fixture.plan = adapters.ProcessPlan{
		Executable: executable,
		Args:       []string{"-test.run=^" + testName + "$"},
		Env: append(os.Environ(),
			windowsPipeDrainRoleEnvironment+"="+role,
			windowsPipeDrainChildReadyEnvironment+"="+fixture.childReady,
			windowsPipeDrainReleaseEnvironment+"="+fixture.release,
			windowsPipeDrainBarrierEnvironment+"="+fixture.barrier,
			windowsPipeDrainPIDEnvironment+"="+fixture.descendant,
		),
	}
	t.Cleanup(func() {
		data, readErr := os.ReadFile(fixture.descendant)
		if readErr != nil {
			if !errors.Is(readErr, os.ErrNotExist) {
				t.Errorf("read Windows pipe-drain descendant PID: %v", readErr)
			}
			return
		}
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr != nil {
			t.Errorf("parse Windows pipe-drain descendant PID %q: %v", data, parseErr)
			return
		}
		process, findErr := os.FindProcess(pid)
		if findErr == nil {
			_ = process.Kill()
		}
	})
	return fixture
}

func releaseWindowsPipeDrainChild(fixture windowsPipeDrainFixture) error {
	if err := waitForFixtureFile(fixture.childReady, 5*time.Second); err != nil {
		return fmt.Errorf("Windows pipe-drain child did not start: %w", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(fixture.release, []byte("release\n"), 0o600); err != nil {
		return fmt.Errorf("release Windows pipe-drain child: %w", err)
	}
	return nil
}

func runWindowsPipeDrainHelper(t *testing.T, testName string) bool {
	t.Helper()
	role := os.Getenv(windowsPipeDrainRoleEnvironment)
	if role == "" {
		return false
	}
	if role == "descendant" {
		if err := os.WriteFile(os.Getenv(windowsPipeDrainPIDEnvironment), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			t.Fatalf("write Windows pipe-drain descendant PID: %v", err)
		}
		time.Sleep(30 * time.Second)
		return true
	}
	if role != "direct-exit" && role != "direct-deadline" {
		t.Fatalf("unknown Windows pipe-drain helper role %q", role)
	}
	if err := os.WriteFile(os.Getenv(windowsPipeDrainChildReadyEnvironment), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatalf("write Windows pipe-drain child readiness: %v", err)
	}
	if err := waitForFixtureFile(os.Getenv(windowsPipeDrainReleaseEnvironment), 5*time.Second); err != nil {
		t.Fatalf("wait for Windows pipe-drain child release: %v", err)
	}
	descendant := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	descendant.Env = append(os.Environ(), windowsPipeDrainRoleEnvironment+"=descendant")
	descendant.Stdout = os.Stdout
	descendant.Stderr = os.Stderr
	if err := descendant.Start(); err != nil {
		t.Fatalf("start Windows pipe-drain descendant: %v", err)
	}
	if err := waitForFixtureFile(os.Getenv(windowsPipeDrainPIDEnvironment), 5*time.Second); err != nil {
		t.Fatalf("wait for Windows pipe-drain descendant: %v", err)
	}
	if _, err := fmt.Fprint(os.Stdout, "AIGW_OK"); err != nil {
		t.Fatalf("write Windows pipe-drain output: %v", err)
	}
	if role == "direct-deadline" {
		if err := os.WriteFile(os.Getenv(windowsPipeDrainBarrierEnvironment), []byte("ready\n"), 0o600); err != nil {
			t.Fatalf("write Windows pipe-drain deadline barrier: %v", err)
		}
		time.Sleep(30 * time.Second)
	}
	return true
}
