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

func TestSourceSizeUsesExactNativeCodeLines(t *testing.T) {
	root := newGitRepository(t)
	policy := filepath.Join(root, ".config", "checks", "scc", "policy.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, readFile(t, filepath.Join("..", "..", ".config", "checks", "scc", "policy.toml")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "foreign-index"))
	for _, test := range []struct {
		name, file, source string
		valid              bool
	}{
		{"boundary", "source.go", "package fixture\n" + strings.Repeat("var _ = 1\n", 499), true},
		{"inline comments", "source.go", "package fixture\n" + strings.Repeat("var _ = 1 // explanation\n", 500), false},
		{"test source", "source_test.go", "package fixture\n" + strings.Repeat("var _ = 1\n", 500), false},
		{"foreign platform", "source_windows.go", "package fixture\n" + strings.Repeat("var _ = 1\n", 500), false},
		{"comment only", "source.go", "package fixture\n" + strings.Repeat("// explanation\n\n", 600), true},
		{"quoted filename", "source's #1.go", "package fixture\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, test.file)
			if err := os.WriteFile(path, []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Remove(path); err != nil {
					t.Error(err)
				}
			})
			err := run([]string{"check-source-size", root}, &bytes.Buffer{}, systemRunner)
			if (err == nil) != test.valid {
				t.Fatalf("source size validity=%t, want %t: %v", err == nil, test.valid, err)
			}
			if !test.valid && (!strings.Contains(err.Error(), test.file) || !strings.Contains(err.Error(), "501 > 500")) {
				t.Fatalf("missing exact file budget finding: %v", err)
			}
			if got := string(readFile(t, path)); got != test.source {
				t.Fatal("measurement changed source")
			}
		})
	}
	if entries, err := os.ReadDir(filepath.Join(root, "build", "tmp")); err != nil || len(entries) != 0 {
		t.Fatalf("measurement retained scratch: %v, %v", entries, err)
	}
}

func TestSourceSizeMeasuresSpecificationsOutsideTransition(t *testing.T) {
	root := newGitRepository(t)
	// Distinct budgets show which one measures each file.
	writeRepositoryFile(t, root, ".config/checks/scc/policy.toml", "max_code_lines = 600\nmax_specification_lines = 500\n")
	writeRepositoryFile(t, root, "source.go", "package fixture\n"+strings.Repeat("var _ = 1\n", 550))
	long := "# Long\n" + strings.Repeat("Line.\n\n", 500)
	short := "# Short\n\nLine.\n"
	writeRepositoryFile(t, root, "openspec/specs/long/spec.md", long)
	writeRepositoryFile(t, root, "openspec/specs/short/spec.md", short)
	writeRepositoryFile(t, root, "openspec/changes/archive/2026-01-01-done/specs/long/spec.md", long)
	check := func() error { return run([]string{"check-source-size", root}, &bytes.Buffer{}, systemRunner) }
	mainSpecification := filepath.Join("openspec", "specs", "long", "spec.md") + ": non-blank lines 501 > 500"
	// An ignored delta is not repository content and exempts nothing.
	writeRepositoryFile(t, root, ".gitignore", "openspec/changes/ignored/\n")
	writeRepositoryFile(t, root, "openspec/changes/ignored/specs/long/spec.md", "## MODIFIED Requirements\n")
	if err := check(); err == nil || !strings.Contains(err.Error(), mainSpecification) || strings.Contains(err.Error(), "archive") || strings.Contains(err.Error(), "source.go") {
		t.Fatalf("the gate did not measure each file against its own budget: %v", err)
	}
	writeRepositoryFile(t, root, "openspec/changes/active/specs/long/spec.md", "## ADDED Requirements\n\n### Requirement: More\n\nText.\n")
	if err := check(); err == nil || !strings.Contains(err.Error(), mainSpecification) {
		t.Fatalf("a delta that only adds requirements exempted its main specification: %v", err)
	}
	writeRepositoryFile(t, root, "openspec/changes/active/specs/long/spec.md", "## MODIFIED Requirements\n\n"+short)
	if err := check(); err != nil {
		t.Fatalf("the gate measured a main specification that an active Change rewrites: %v", err)
	}
	writeRepositoryFile(t, root, "openspec/changes/active/specs/long/spec.md", "## REMOVED Requirements\n"+strings.Repeat("Line.\n\n", 500))
	delta := filepath.Join("openspec", "changes", "active", "specs", "long", "spec.md") + ": non-blank lines 501 > 500"
	if err := check(); err == nil || !strings.Contains(err.Error(), delta) {
		t.Fatalf("a delta above the limit passed: %v", err)
	}
}

func TestSourceSizeRequiresCompleteMeasurement(t *testing.T) {
	budgets := map[string]sizeBudget{"first.go": {maximum: 500, unit: "code lines"}, "second.go": {maximum: 500, unit: "code lines"}}
	for _, test := range []struct{ name, output, diagnostic string }{
		{"complete", `[{"Files":[{"Location":"first.go","Code":1},{"Location":"second.go","Code":2}]}]`, ""},
		{"missing", `[{"Files":[{"Location":"first.go","Code":1}]}]`, "second.go"},
		{"foreign", `[{"Files":[{"Location":"foreign.go","Code":1}]}]`, "foreign.go"},
		{"duplicate", `[{"Files":[{"Location":"first.go","Code":1},{"Location":"first.go","Code":1}]}]`, "first.go"},
		{"missing count", `[{"Files":[{"Location":"first.go"}]}]`, "first.go"},
		{"negative", `[{"Files":[{"Location":"first.go","Code":-1}]}]`, "first.go"},
		{"malformed", `{`, "decode"},
		{"empty", `[]`, "first.go"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateSourceSizes([]byte(test.output), budgets)
			if test.diagnostic == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.diagnostic) {
				t.Fatalf("incomplete measurement accepted: %v", err)
			}
		})
	}
}

func TestSourceSizeStopsOnNativeFailure(t *testing.T) {
	root := newGitRepository(t)
	policy := filepath.Join(root, ".config", "checks", "scc", "policy.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("max_code_lines = 500\nmax_specification_lines = 500\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("native measurement failed")
	err := checkSourceSize(root, func(call command) error {
		if call.Name != "scc" || call.Dir != root || len(call.Args) != 1 || !strings.HasPrefix(call.Args[0], "@") {
			t.Fatalf("native command expanded source inventory: %+v", call)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("native failure was lost: %v", err)
	}
	for _, body := range []string{
		"max_code_lines = 0\nmax_specification_lines = 500", "max_code_lines = -1\nmax_specification_lines = 500",
		"max_code_lines = 500", "max_code_lines = 500\nmax_specification_lines = 0",
		"max_code_lines = 500\nmax_specification_lines = 500\nunknown = true", "invalid = [",
	} {
		if err := os.WriteFile(policy, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		err := checkSourceSize(root, func(command) error { return fmt.Errorf("unexpected native invocation") })
		if err == nil || strings.Contains(err.Error(), "unexpected native invocation") {
			t.Fatalf("invalid policy reached measurement: %q, %v", body, err)
		}
	}
}
