package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealMainRejectsUnavailableTemporaryDirectory(t *testing.T) {
	// TMPDIR is not honored by os.CreateTemp on Windows (it uses
	// TMP/TEMP/USERPROFILE instead), so the temporary-directory failure is
	// forced directly through createCoverageProfile instead of relying on
	// a platform-specific environment variable.
	missing := filepath.Join(t.TempDir(), "missing")
	restore := stubCoverageProfile(func() (*os.File, error) {
		return os.CreateTemp(missing, "aigw-coverage-*.out")
	})
	defer restore()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, &recordingRunner{}); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "create coverage profile") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsCoverageProfileCloseFailure(t *testing.T) {
	restore := stubCoverageProfile(func() (*os.File, error) {
		file, err := os.CreateTemp(t.TempDir(), "aigw-coverage-*.out")
		if err != nil {
			return nil, err
		}
		// Close it early so the Close call inside realMain observes the
		// already-closed error deterministically on every platform.
		if err := file.Close(); err != nil {
			return nil, err
		}
		return file, nil
	})
	defer restore()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, &recordingRunner{}); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "close coverage profile") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCoverageCleanupFailurePreventsSuccessfulExit(t *testing.T) {
	for _, failure := range []error{nil, errors.New("test process failed")} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			directory := t.TempDir()
			remove := removeCoverageProfile
			t.Cleanup(func() { removeCoverageProfile = remove })
			var removed string
			removeCoverageProfile = func(path string) error {
				removed = path
				return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
			}
			var profilePath string
			restore := stubCoverageProfile(func() (*os.File, error) {
				profile, err := os.CreateTemp(directory, "coverage-")
				if err != nil {
					return nil, err
				}
				profilePath = profile.Name()
				return profile, nil
			})
			defer restore()
			var stdout, stderr bytes.Buffer
			runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 100 1\n", err: failure}
			code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, runner)
			if code != 1 || removed != profilePath || !strings.Contains(stderr.String(), "remove coverage profile") || !strings.Contains(stderr.String(), profilePath) {
				t.Fatalf("cleanup failed without a failing result: code=%d, %s", code, stderr.String())
			}
			if failure != nil && !strings.Contains(stderr.String(), failure.Error()) {
				t.Fatalf("cleanup hid the test failure: %s", stderr.String())
			}
		})
	}
}

func stubCoverageProfile(factory func() (*os.File, error)) func() {
	original := createCoverageProfile
	createCoverageProfile = factory
	return func() { createCoverageProfile = original }
}
