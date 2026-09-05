package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const captureFailureFixture = "AIGW_TEST_CAPTURE_FAILURE_FIXTURE"

func TestLimitedBufferWriteEnforcesLimitAndFlagsOverflow(t *testing.T) {
	buf := &limitedBuffer{limit: 8}
	n, err := buf.Write([]byte("1234"))
	if err != nil || n != 4 || buf.overflow {
		t.Fatalf("first write = (%d, %v), overflow=%v", n, err, buf.overflow)
	}
	n, err = buf.Write([]byte("567890"))
	if n != 4 || !errors.Is(err, errCapturedProcessOutputLimit) || !buf.overflow {
		t.Fatalf("partial overflow write = (%d, %v), overflow=%v", n, err, buf.overflow)
	}
	if buf.String() != "12345678" {
		t.Fatalf("buffer contents = %q, want truncated at the limit", buf.String())
	}
	n, err = buf.Write([]byte("x"))
	if n != 0 || !errors.Is(err, errCapturedProcessOutputLimit) {
		t.Fatalf("write past a full buffer = (%d, %v)", n, err)
	}
}

func TestRunnerRunRejectsMissingExecutable(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil {
		t.Fatal("Run() with a missing executable should fail")
	}
}

func TestRunnerRunCaptureSurfacesStartFailure(t *testing.T) {
	_, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil || !strings.Contains(err.Error(), "start ") {
		t.Fatalf("RunCapture() error = %v", err)
	}
}

func TestRunnerRunCaptureReturnsStderrOnFailure(t *testing.T) {
	if os.Getenv(captureFailureFixture) == "child" {
		_, _ = os.Stderr.WriteString("Error loading config.toml: unknown configuration field mcp_servers.github.disabled_reason\n")
		os.Exit(23)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: executable,
		Args:       []string{"-test.run=^TestRunnerRunCaptureReturnsStderrOnFailure$"},
		Env:        append(os.Environ(), captureFailureFixture+"=child"),
	})
	var exitError *exec.ExitError
	if err == nil || !errors.As(err, &exitError) {
		t.Fatalf("RunCapture() error = %v, want preserved process exit", err)
	}
	if got := string(diagnostic); !strings.Contains(got, "unknown configuration field mcp_servers.github.disabled_reason") {
		t.Fatalf("RunCapture() diagnostic = %q", got)
	}
}
