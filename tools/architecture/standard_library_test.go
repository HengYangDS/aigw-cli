package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// Every run applies the rule: a listed package that imports another package of
// the module fails the gate.
func TestRunEnforcesStandardLibraryOnly(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy+"standard_library_only = [\"internal/model\"]\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module fixture\n")
	writeFile(t, filepath.Join(root, "internal", "model", "model.go"), "// Package model owns the fixture model.\npackage model\n\nimport _ \"fixture/internal/store\"\n")
	writeFile(t, filepath.Join(root, "internal", "store", "store.go"), "// Package store owns the fixture store.\npackage store\n")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d want 1 stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	if report := decodeReport(t, stdout.String()); !hasRule(report, "standard_library_only") {
		t.Fatalf("the run did not apply standard_library_only: %v", findingRules(report))
	}
}

func TestStandardLibraryOnlyRefusesModuleAndThirdPartyImports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	writeFile(t, filepath.Join(root, "internal", "domain", "model.go"), "package domain\n\nimport (\n\t\"encoding/json\"\n\t\"strings\"\n\t\"example.com/aigw/internal/store\"\n\t\"github.com/pelletier/go-toml/v2\"\n\t\"C\"\n)\n")
	writeFile(t, filepath.Join(root, "internal", "domain", "model_test.go"), "package domain\n\nimport _ \"github.com/pelletier/go-toml/v2\"\n")
	writeFile(t, filepath.Join(root, "internal", "store", "store.go"), "package store\n\nimport _ \"github.com/pelletier/go-toml/v2\"\n")
	files := []goFileInfo{
		{relPath: "internal/domain/model.go", dir: "internal/domain"},
		{relPath: "internal/domain/model_test.go", dir: "internal/domain", isTest: true},
		{relPath: "internal/store/store.go", dir: "internal/store"},
	}
	report := newReport("policy", root)
	if err := checkStandardLibraryOnly(root, files, policy{StandardLibraryOnly: []string{"internal/domain"}}, &report); err != nil {
		t.Fatal(err)
	}
	var refused []string
	for _, finding := range report.Findings {
		if finding.Rule == "standard_library_only" {
			refused = append(refused, finding.Message)
		}
	}
	joined := strings.Join(refused, "\n")
	for _, want := range []string{`"example.com/aigw/internal/store"`, `"github.com/pelletier/go-toml/v2"`, `"C"`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("import %s was admitted: %v", want, refused)
		}
	}
	if len(refused) != 3 || countRule(report, "standard_library_only_entry") != 0 {
		t.Fatalf("findings = %+v; want exactly the three non-standard imports of the listed production file", report.Findings)
	}
}

func TestStandardLibraryOnlyReportsAStaleEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	writeFile(t, filepath.Join(root, "internal", "fixtures", "fixture_test.go"), "package fixtures\n")
	// A package with only test files has no production Go files either.
	files := []goFileInfo{{relPath: "internal/fixtures/fixture_test.go", dir: "internal/fixtures", isTest: true}}
	report := newReport("policy", root)
	if err := checkStandardLibraryOnly(root, files, policy{StandardLibraryOnly: []string{"internal/retired", "internal/fixtures"}}, &report); err != nil {
		t.Fatal(err)
	}
	if countRule(report, "standard_library_only_entry") != 2 {
		t.Fatalf("a listed package without production files was accepted: %+v", report.Findings)
	}
}

func TestStandardLibraryRecognizesOnlyStandardPaths(t *testing.T) {
	for importPath, want := range map[string]bool{
		"strings":               true,
		"encoding/json":         true,
		"aigw-cli":              false,
		"aigw-cli/internal/x":   false,
		"github.com/owner/repo": false,
		"golang.org/x/tools":    false,
		"C":                     false,
	} {
		if got := isStandardLibrary(importPath, "aigw-cli"); got != want {
			t.Errorf("isStandardLibrary(%q) = %v, want %v", importPath, got, want)
		}
	}
}

func TestPolicyRejectsInvalidStandardLibraryOnlyEntries(t *testing.T) {
	for _, entries := range [][]string{{"internal/domain", "internal/domain"}, {"../domain"}, {""}} {
		p := policy{GoRoots: []string{"internal"}, StandardLibraryOnly: entries}
		if err := validatePackagePolicy(p); err == nil || !strings.Contains(err.Error(), "standard_library_only") {
			t.Fatalf("entries %q were accepted: %v", entries, err)
		}
	}
}
