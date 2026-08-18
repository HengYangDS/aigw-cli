package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckProtectedLifecycleAcceptsArchivedOnlyChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "archive", "2026-08-18-done"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkProtectedLifecycle(root); err != nil {
		t.Fatal(err)
	}
}

func TestCheckProtectedLifecycleRejectsActiveChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "active-change"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := checkProtectedLifecycle(root)
	if err == nil || !strings.Contains(err.Error(), "active-change") {
		t.Fatalf("protected lifecycle error = %v", err)
	}
}

func TestCheckProtectedLifecycleAllowsAuthoringLanes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "active-change"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "--quiet", root}, {"-C", root, "switch", "-c", "work/example"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := checkProtectedLifecycle(root); err != nil {
		t.Fatalf("authoring lane rejected: %v", err)
	}
}
