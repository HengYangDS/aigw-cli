//go:build darwin

package construction

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReleaseBuildPreservesToolAndWorkspaceCleanupFailures(t *testing.T) {
	root := releaseRoot(t)
	output := filepath.Join(root, "dist")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	accepted := filepath.Join(output, "accepted")
	if err := os.WriteFile(accepted, []byte("previous release"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("interrupted release tool")
	var workspace string
	err := buildRelease(buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: "fixture-key"}, func(call toolCall) error {
		if call.Name != "goreleaser" {
			return nil
		}
		workspace = filepath.Dir(goReleaserStage(t, call.Args))
		if err := unix.Chflags(workspace, unix.UF_IMMUTABLE); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := unix.Chflags(workspace, 0); err != nil {
				t.Errorf("restore fixture permissions: %v", err)
			}
		})
		return want
	})
	if !errors.Is(err, want) || !errors.Is(err, unix.EPERM) || !strings.Contains(err.Error(), workspace) {
		t.Fatalf("release discarded tool or cleanup cause and location: %v", err)
	}
	if content, err := os.ReadFile(accepted); err != nil || string(content) != "previous release" {
		t.Fatalf("failed build changed accepted release: %q, %v", content, err)
	}
}

func TestReleaseOutputReportsPublicationBeforeCleanupFailure(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "candidate"), filepath.Join(root, "dist")
	for directory, content := range map[string]string{source: "new release", target: "old release"} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "artifact"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	previous, err := os.Open(filepath.Join(target, "artifact"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(unix.Fchflags(int(previous.Fd()), 0), previous.Close()); err != nil {
			t.Errorf("restore retained fixture permissions: %v", err)
		}
	})
	if err := unix.Fchflags(int(previous.Fd()), unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	err = replaceDirectory(source, target)
	if !errors.Is(err, unix.EPERM) || !strings.Contains(err.Error(), "release output published") {
		t.Fatalf("completed publication was misreported: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(target, "artifact")); err != nil || string(content) != "new release" {
		t.Fatalf("published artifact = %q, %v", content, err)
	}
	retained, err := filepath.Glob(filepath.Join(root, ".aigw-release-backup-*", "previous", "artifact"))
	if err != nil || len(retained) != 1 {
		t.Fatalf("expected one exact retained predecessor: %v, %v", retained, err)
	}
	if content, err := os.ReadFile(retained[0]); err != nil || string(content) != "old release" {
		t.Fatalf("retained predecessor = %q, %v", content, err)
	}
}
