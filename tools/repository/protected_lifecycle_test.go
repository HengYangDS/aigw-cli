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
	if err := checkProtectedLifecycleForRef(root, "main"); err != nil {
		t.Fatal(err)
	}
}

func TestCheckProtectedLifecycleReportsMissingWorkspace(t *testing.T) {
	err := checkProtectedLifecycleForRef(t.TempDir(), "main")
	if err == nil || !strings.Contains(err.Error(), "read OpenSpec changes") {
		t.Fatalf("protected lifecycle error = %v", err)
	}
}

func TestCheckProtectedLifecycleRejectsActiveChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "active-change"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := checkProtectedLifecycleForRef(root, "main")
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
	if err := checkProtectedLifecycleForRef(root, repositoryBranch(root)); err != nil {
		t.Fatalf("authoring lane rejected: %v", err)
	}
}

func TestCheckProtectedLifecycleAllowsDetachedForgeProposal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "active-change"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		variable string
	}{
		{name: "GitLab branch pipeline", variable: "CI_COMMIT_BRANCH"},
		{name: "GitLab merge request", variable: "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"},
		{name: "GitHub pull request", variable: "GITHUB_HEAD_REF"},
		{name: "GitHub branch pipeline", variable: "GITHUB_REF_NAME"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, variable := range []string{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "CI_COMMIT_BRANCH", "GITHUB_HEAD_REF", "GITHUB_REF_NAME"} {
				t.Setenv(variable, "")
			}
			t.Setenv(test.variable, "proposal/example")
			if err := checkProtectedLifecycle(root); err != nil {
				t.Fatalf("detached proposal rejected: %v", err)
			}
		})
	}
}

func TestPublicationRefFallsBackToTheCheckedOutBranch(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "--quiet", root}, {"-C", root, "switch", "-c", "work/fallback"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	for _, variable := range []string{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "CI_COMMIT_BRANCH", "GITHUB_HEAD_REF", "GITHUB_REF_NAME"} {
		t.Setenv(variable, "")
	}
	if got := publicationRef(root); got != "work/fallback" {
		t.Fatalf("publicationRef() = %q, want work/fallback", got)
	}
}
