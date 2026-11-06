package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRealMainRejectsUnavailableTemporaryDirectory(t *testing.T) {
	usePolicy(t, validPolicy)
	missing := filepath.Join(t.TempDir(), "missing")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, missing)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain(nil, &stdout, &stderr, &recordingRunner{}); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "create coverage profile") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// undeletableProfileRunner runs as recordingRunner does, and then keeps the
// profile from being removed until the test ends: Windows refuses to delete an
// open file, and other hosts refuse to delete an entry of a directory without
// write permission. The profile stays readable.
type undeletableProfileRunner struct {
	recordingRunner
	t *testing.T
}

func (r *undeletableProfileRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	err := r.recordingRunner.Run(name, args, stdout, stderr)
	if args[0] != "test" {
		return err
	}
	held, openErr := os.Open(r.path)
	if openErr != nil {
		r.t.Fatal(openErr)
	}
	directory := filepath.Dir(r.path)
	if chmodErr := os.Chmod(directory, 0o500); chmodErr != nil {
		r.t.Fatal(chmodErr)
	}
	r.t.Cleanup(func() {
		_ = held.Close()
		_ = os.Chmod(directory, 0o700)
	})
	return err
}

func TestCoverageCleanupFailurePreventsSuccessfulExit(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("root removes an entry of a directory without write permission")
	}
	for _, failure := range []error{nil, errors.New("test process failed")} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			usePolicy(t, validPolicy)
			scratch := t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, scratch)
			}
			var stdout, stderr bytes.Buffer
			runner := &undeletableProfileRunner{t: t, profile: "mode: atomic\nexample/a.go:1.1,2.1 100 1\n", err: failure}
			code := realMain(nil, &stdout, &stderr, runner)
			if code != 1 || !strings.Contains(stderr.String(), "remove coverage profile") || !strings.Contains(stderr.String(), runner.path) {
				t.Fatalf("cleanup failed without a failing result: code=%d, %s", code, stderr.String())
			}
			if failure == nil && !strings.Contains(stdout.String(), "statement coverage: 100.00%") {
				t.Fatalf("the run failed before its cleanup: %s, %s", stdout.String(), stderr.String())
			}
			if failure != nil && !strings.Contains(stderr.String(), failure.Error()) {
				t.Fatalf("cleanup hid the test failure: %s", stderr.String())
			}
		})
	}
}
