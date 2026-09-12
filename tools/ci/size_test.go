package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceSizeUsesExactNativeCodeLines(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	policy := filepath.Join(root, ".config", "checks", "go", "size.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("max_code_lines = 500\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
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

func TestSourceSizeRequiresCompleteMeasurement(t *testing.T) {
	files := []string{"first.go", "second.go"}
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
			err := validateSourceSizes([]byte(test.output), files, 500)
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
	root := t.TempDir()
	policy := filepath.Join(root, ".config", "checks", "go", "size.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("max_code_lines = 500\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
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
	for _, body := range []string{"max_code_lines = 0", "max_code_lines = -1", "max_code_lines = 500\nunknown = true", "invalid = ["} {
		if err := os.WriteFile(policy, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		err := checkSourceSize(root, func(command) error { return fmt.Errorf("unexpected native invocation") })
		if err == nil || strings.Contains(err.Error(), "unexpected native invocation") {
			t.Fatalf("invalid policy reached measurement: %q, %v", body, err)
		}
	}
}
