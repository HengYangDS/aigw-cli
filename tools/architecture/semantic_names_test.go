package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestSemanticNamesAcceptNativeCarrierGrammars(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n")
	writeFile(t, filepath.Join(root, "docs", "release-policy.md"), "# Policy\n")
	writeFile(t, filepath.Join(root, "internal", "routing", "route_plan.go"), "package routing\n")
	writeFile(t, filepath.Join(root, "scripts", "checks", "check-release.sh"), "#!/bin/sh\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestSemanticNamesAcceptDatedChronicleCarrier(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "evidence", "chronicle", "product-convergence", "2026-07-31.md"), "# Chronicle\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("dated chronicle findings=%+v", report.Findings)
	}
}

func TestSemanticNamesIgnoreUntrackedHostFiles(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n")
	runGit(t, root, "add", "README.md")
	writeFile(t, filepath.Join(root, "2026-scratch.md"), "# Host scratch\n")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("untracked host files are outside repository-owned state: findings=%+v", report.Findings)
	}
}

func TestSemanticNamesRejectWrongCarrierAndNumericNames(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "docs", "2026-plan.md"), "# Plan\n")
	writeFile(t, filepath.Join(root, "internal", "routing", "route-plan.go"), "package routing\n")
	writeFile(t, filepath.Join(root, "scripts", "checks", "check_release.sh"), "#!/bin/sh\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "semantic_name_markdown", "docs/2026-plan.md")
	assertFinding(t, report.Findings, "semantic_name_go", "internal/routing/route-plan.go")
	assertFinding(t, report.Findings, "semantic_name_shell", "scripts/checks/check_release.sh")
}

func TestSemanticNamesRejectPythonFiles(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "scripts", "checks", "legacy.py"), "print('legacy')\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "python_source", "scripts/checks/legacy.py")
}

func TestSemanticNamesRejectEmbeddedPythonExecution(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "scripts", "checks", "check-release.sh"), "#!/bin/sh\npython3 - <<'PY'\nprint('legacy')\nPY\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "python_execution", "scripts/checks/check-release.sh")
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
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatalf("deleted tracked paths are absent from the workspace: %v", err)
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

func TestSemanticNamesDetectPortableTextBindings(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "docs", "bindings.md"), "/Users/alice/project\nC:\\\\Users\\\\alice\\\\project\n10.0.0.1\n$HOME/.ssh/id_ed25519\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"absolute_user_home", "absolute_windows_user_home", "private_ipv4", "personal_ssh_path"} {
		if !hasRule(report, rule) {
			t.Fatalf("missing %s in %+v", rule, report.Findings)
		}
	}
}

func TestSemanticNamesIgnoreFixtureBindingsAndPythonComments(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "scripts", "tests", "binding-fixture.sh"), "#!/bin/sh\n# python3 is fixture prose\nprintf '%s\\n' /Users/alice/project\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("fixture and comments must not produce findings: %+v", report.Findings)
	}
}

func TestRepositoryTextChecksReportUnavailableInputs(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	report := newReport("policy", missing)
	if err := checkSemanticNames(missing, &report); err == nil {
		t.Fatal("missing repository accepted by semantic-name check")
	}
	if err := checkTextLayout(missing, &report); err == nil {
		t.Fatal("missing repository accepted by text-layout check")
	}
	if err := checkPythonExecution(missing, "missing.sh", &report); err == nil {
		t.Fatal("missing shell carrier accepted")
	}
	if err := checkPortableText(missing, "missing.md", &report); err != nil {
		t.Fatalf("missing portable-text carrier: %v", err)
	}
}
