package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const captureFailureFixture = "AIGW_TEST_CAPTURE_FAILURE_FIXTURE"

func TestRunCaptureKeepsResultAndDiagnosticBudgetsSeparate(t *testing.T) {
	if stream := os.Getenv("AIGW_TEST_CAPTURE_STREAM"); stream != "" {
		size, err := strconv.Atoi(os.Getenv("AIGW_TEST_CAPTURE_SIZE"))
		if err != nil {
			os.Exit(2)
		}
		target := os.Stdout
		if stream == "stderr" {
			target = os.Stderr
		}
		if _, err := target.WriteString(strings.Repeat("x", size)); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		limit  int
		size   int
		stream string
		accept bool
	}{
		{"default result", 0, capturedProcessOutputLimit + 1, "stdout", false},
		{"larger result", capturedProcessOutputLimit * 2, capturedProcessOutputLimit + 1, "stdout", true},
		{"exact result budget", capturedProcessOutputLimit * 2, capturedProcessOutputLimit * 2, "stdout", true},
		{"result overflow", capturedProcessOutputLimit * 2, capturedProcessOutputLimit*2 + 1, "stdout", false},
		{"separate diagnostics", 0, 128, "stderr", true},
		{"diagnostic remains bounded", capturedProcessOutputLimit * 2, capturedProcessOutputLimit + 1, "stderr", false},
		{"invalid budget", -1, 1, "stdout", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := (Runner{StdoutLimit: test.limit}).RunCapture(t.Context(), Plan{
				Executable: executable,
				Args:       []string{"-test.run=^TestRunCaptureKeepsResultAndDiagnosticBudgetsSeparate$"},
				Env: append(os.Environ(),
					"AIGW_TEST_CAPTURE_STREAM="+test.stream,
					"AIGW_TEST_CAPTURE_SIZE="+strconv.Itoa(test.size)),
			})
			if test.accept {
				want := test.size
				if test.stream == "stderr" {
					want = 0
				}
				if err != nil || len(output) != want {
					t.Fatalf("captured %d bytes, want %d: %v", len(output), want, err)
				}
			} else if err == nil || len(output) != 0 {
				t.Fatalf("invalid capture returned %d bytes: %v", len(output), err)
			}
		})
	}
}

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

func TestRunnerRunToFileRejectsUnwritableDestination(t *testing.T) {
	runner := Runner{}
	destination := filepath.Join(t.TempDir(), "missing", "out")
	if err := runner.RunToFile(context.Background(), destination, Plan{Executable: "echo", Args: []string{"hi"}}); err == nil || !strings.Contains(err.Error(), "open command output") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunnerRejectsOversizedStreamDiagnostics(t *testing.T) {
	if os.Getenv("AIGW_TEST_STREAM_OVERFLOW") == "1" {
		_, _ = os.Stderr.WriteString(strings.Repeat("x", 128<<10))
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "asset")
	err = (Runner{}).RunToFile(t.Context(), destination, Plan{
		Executable: executable,
		Args:       []string{"-test.run=^TestRunnerRejectsOversizedStreamDiagnostics$"},
		Env:        append(os.Environ(), "AIGW_TEST_STREAM_OVERFLOW=1"),
	})
	if err == nil {
		t.Fatal("file capture accepted an incomplete diagnostic stream")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed capture retained an asset: %v", err)
	}
}

func TestRunnerStreamsOutputWithoutCaptureCeiling(t *testing.T) {
	if os.Getenv("AIGW_TEST_FILE_OUTPUT") == "1" {
		output, err := os.Stdout.Stat()
		if err != nil || output.Mode()&os.ModeNamedPipe == 0 {
			_, _ = os.Stderr.WriteString("file capture must retain destination ownership in the parent")
			os.Exit(24)
		}
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 128<<10))
		code, _ := strconv.Atoi(os.Getenv("AIGW_TEST_FILE_EXIT"))
		if code != 0 {
			_, _ = os.Stderr.WriteString("download failed")
		}
		os.Exit(code)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, exit := range []int{0, 23} {
		t.Run(strconv.Itoa(exit), func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "asset")
			err := (Runner{}).RunToFile(t.Context(), destination, Plan{
				Executable: executable,
				Args:       []string{"-test.run=^TestRunnerStreamsOutputWithoutCaptureCeiling$"},
				Env:        append(os.Environ(), "AIGW_TEST_FILE_OUTPUT=1", "AIGW_TEST_FILE_EXIT="+strconv.Itoa(exit)),
			})
			if exit != 0 {
				var failure *exec.ExitError
				if !errors.As(err, &failure) || failure.ExitCode() != exit || !strings.Contains(err.Error(), "download failed") {
					t.Fatalf("file capture failure = %v", err)
				}
				if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("partial output survived: %v", statErr)
				}
				return
			}
			data, readErr := os.ReadFile(destination)
			if err != nil || readErr != nil || string(data) != strings.Repeat("x", 128<<10) {
				t.Fatalf("file output = %d bytes, run %v, read %v", len(data), err, readErr)
			}
		})
	}
}

func TestRunnerPreservesPlanEnvironment(t *testing.T) {
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "environment-fixture" {
		_, _ = fmt.Fprint(os.Stdout, os.Getenv("AIGW_TEST_ENV_CONTRACT"))
		os.Exit(0)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_TEST_ENV_CONTRACT", "inherited")
	for _, environment := range []string{"inherited", "empty", "explicit"} {
		for _, output := range []string{"capture", "file"} {
			t.Run(environment+"/"+output, func(t *testing.T) {
				temporary := t.TempDir()
				for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
					t.Setenv(name, temporary)
				}
				plan := Plan{Executable: program, Args: []string{"-test.run=^TestRunnerPreservesPlanEnvironment$", "environment-fixture"}}
				want := "inherited"
				switch environment {
				case "empty":
					plan.Env, want = []string{}, ""
				case "explicit":
					plan.Env = []string{"AIGW_TEST_ENV_CONTRACT=explicit", "TMPDIR=" + temporary, "TMP=" + temporary, "TEMP=" + temporary}
					want = "explicit"
				}
				var data []byte
				if output == "capture" {
					data, err = (Runner{}).RunCapture(t.Context(), plan)
				} else {
					path := filepath.Join(t.TempDir(), "output")
					err = (Runner{}).RunToFile(t.Context(), path, plan)
					if err == nil {
						data, err = os.ReadFile(path)
					}
				}
				if err != nil || string(data) != want {
					t.Fatalf("environment output = %q, want %q, error = %v", data, want, err)
				}
				entries, readErr := os.ReadDir(temporary)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("fixture temporary entries = %v, error = %v", entries, readErr)
				}
			})
		}
	}
}
