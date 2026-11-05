package main

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// trackFixture makes root a Git checkout whose index holds every fixture file,
// whatever the host's ignore rules, as the analyzed repository's index does.
func trackFixture(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init", "-q")
	runGit(t, root, "add", "--all", "--force")
}

func trackedFixture(t *testing.T, root string) []string {
	t.Helper()
	files, err := trackedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestTrackedInventoryRequiresTheRequestedCheckout(t *testing.T) {
	parent := t.TempDir()
	runGit(t, parent, "init", "-q")
	root := filepath.Join(parent, "fixture")
	writeFile(t, filepath.Join(root, "owned.go"), "package fixture\n")
	runGit(t, parent, "add", "--all")
	if files, err := trackedFiles(root); err == nil || !strings.Contains(err.Error(), "list tracked files") {
		t.Fatalf("a root without its own checkout read its parent's index: %v, %v", files, err)
	}
}

func TestTrackedInventoryUsesTheRequestedCheckoutIndex(t *testing.T) {
	root, foreign := t.TempDir(), t.TempDir()
	for _, directory := range []string{root, foreign} {
		runGit(t, directory, "init", "-q")
		writeFile(t, filepath.Join(directory, "README.md"), "# Project\n")
		runGit(t, directory, "add", "README.md")
	}
	writeFile(t, filepath.Join(root, "owned.md"), "# Owned source\n")
	writeFile(t, filepath.Join(root, "untracked.md"), "# Local notes\n")
	runGit(t, root, "add", "owned.md")
	for _, binding := range []struct{ name, value string }{
		{"GIT_INDEX_FILE", filepath.Join(foreign, ".git", "index")},
		{"GIT_COMMON_DIR", filepath.Join(foreign, ".git")},
	} {
		t.Run(binding.name, func(t *testing.T) {
			t.Setenv(binding.name, binding.value)
			files, err := trackedFiles(root)
			if err != nil || !slices.Equal(files, []string{"README.md", "owned.md"}) {
				t.Fatalf("requested checkout inventory = %v, %v", files, err)
			}
		})
	}
}

func TestSemanticNamesAcceptNativeCarrierGrammars(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n")
	writeFile(t, filepath.Join(root, "docs", "release-policy.md"), "# Policy\n")
	writeFile(t, filepath.Join(root, "internal", "routing", "route_plan.go"), "package routing\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	checkSemanticNames(trackedFixture(t, root), &report)
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestSemanticNamesIgnoreUntrackedHostFiles(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n")
	runGit(t, root, "add", "README.md")
	writeFile(t, filepath.Join(root, "2026-scratch.md"), "# Host scratch\n")
	report := newReport("policy", root)
	checkSemanticNames(trackedFixture(t, root), &report)
	if !report.OK {
		t.Fatalf("untracked host files are outside repository-owned state: findings=%+v", report.Findings)
	}
}

func TestSemanticNamesRejectWrongCarrierAndNumericNames(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "docs", "2026-plan.md"), "# Plan\n")
	writeFile(t, filepath.Join(root, "internal", "routing", "route-plan.go"), "package routing\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	checkSemanticNames(trackedFixture(t, root), &report)
	assertFinding(t, report.Findings, "semantic_name_markdown", "docs/2026-plan.md")
	assertFinding(t, report.Findings, "semantic_name_go", "internal/routing/route-plan.go")
}

func TestSemanticNamesAcceptLanguageIndependentCarriers(t *testing.T) {
	for _, test := range []struct{ name, content string }{
		{"legacy.py", "print('legacy')\n"},
		{"check-release.sh", "#!/bin/sh\npython3 - <<'PY'\nprint('legacy')\nPY\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			runGit(t, root, "init", "-q")
			writeFile(t, filepath.Join(root, "scripts", "checks", test.name), test.content)
			runGit(t, root, "add", ".")
			report := newReport("policy", root)
			checkSemanticNames(trackedFixture(t, root), &report)
			if !report.OK {
				t.Fatalf("language choice alone was treated as a repository defect: %+v", report.Findings)
			}
		})
	}
}

func TestSemanticNamesIgnoreTrackedDeletion(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	removed := filepath.Join(root, "tools", "legacy", "main.go")
	writeFile(t, removed, "package main\n")
	runGit(t, root, "add", ".")
	if err := os.Remove(removed); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	checkSemanticNames(trackedFixture(t, root), &report)
	if !report.OK {
		t.Fatalf("deleted tracked paths are absent from the workspace: %+v", report.Findings)
	}
}

func TestTrackedFilesOmitsDeletedIndexEntries(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	removed := filepath.Join(root, "internal", "routing", "removed.go")
	writeFile(t, removed, "package routing\n")
	runGit(t, root, "add", ".")
	if err := os.Remove(removed); err != nil {
		t.Fatal(err)
	}
	files, err := trackedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("deleted index entries remained in the source inventory: %v", files)
	}
}

func TestSemanticNamesRecognizeOpenSpecCarriers(t *testing.T) {
	for _, name := range []string{"design.md", "proposal.md", "spec.md", "tasks.md"} {
		if !isOpenSpecCarrier("openspec/changes/example/"+name, name) {
			t.Fatalf("%s must be a native OpenSpec carrier", name)
		}
	}
	if isOpenSpecCarrier("docs/spec.md", "spec.md") || isOpenSpecCarrier("openspec/notes.md", "notes.md") {
		t.Fatal("only canonical OpenSpec carrier names are exempt")
	}
}

func TestReadModuleIdentityReportsScannerFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.mod")
	line := make([]byte, 64*1024+1)
	for index := range line {
		line[index] = 'x'
	}
	if err := os.WriteFile(path, line, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readModuleIdentity(path); !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("error = %v", err)
	}
}
