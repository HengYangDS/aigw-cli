package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestOpenSpecCheckRequiresTheRepositoryLocalDependency(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	checker := filepath.Join(root, "node_modules", "@fission-ai", "openspec", "bin", "openspec.js")
	output, err := systemOutputRunner(command{Name: "node", Args: []string{checker, "validate", "--all", "--strict"}})
	if err == nil || !bytes.Contains(output, []byte("MODULE_NOT_FOUND")) || !bytes.Contains(output, []byte("node_modules")) {
		t.Fatalf("missing local dependency: error=%v output=%s", err, output)
	}
}

func TestTOMLChecksExecuteInRequestedRepository(t *testing.T) {
	repository := repositoryRoot(t)
	caller := t.TempDir()
	root := filepath.Join(caller, "checkout with spaces")
	valid := `z = 2
a = 1
choices = [
  "z",
  "a",
]
record = {z = 2, a = 1}
`
	lockValid := strings.ReplaceAll(valid, "{z = 2, a = 1}", "{ z = 2, a = 1 }")
	files := map[string]string{
		"mise.lock":                 lockValid,
		"tools/domain/config.toml":  valid,
		"internal/domain/test.toml": valid,
		"build/generated.toml":      "value = [\n",
	}
	for _, path := range []string{".gitignore", ".config/checks/toml/policy.toml"} {
		files[path] = string(readFile(t, filepath.Join(repository, filepath.FromSlash(path))))
	}
	for path, content := range files {
		destination := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	t.Chdir(caller)
	for _, test := range []struct {
		name, root, path, content string
		valid                     bool
	}{
		{"absolute root", root, "mise.lock", lockValid, true},
		{"relative root", filepath.Base(root), "mise.lock", lockValid, true},
		{"native lock spacing", root, "mise.lock", valid, false},
		{"authored table spacing", root, "tools/domain/config.toml", lockValid, false},
		{"source fixture syntax", root, "internal/domain/test.toml", "value = [\n", false},
		{"tool policy format", root, "tools/domain/config.toml", "value=1\n", false},
		{"lock syntax", root, "mise.lock", "value = [\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			destination := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.WriteFile(destination, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.WriteFile(destination, []byte(files[test.path]), 0o600); err != nil {
					t.Error(err)
				}
			})
			var output []byte
			var inputs []string
			err := run([]string{"check-toml", test.root}, &bytes.Buffer{}, func(call command) error {
				if call.Dir != root || !slices.Equal(call.Env, []string{"RUST_LOG=warn"}) {
					t.Fatalf("TOML subprocess contract: %#v", call)
				}
				inputs = append(inputs, call.Args[slices.Index(call.Args, "--")+1:]...)
				var err error
				output, err = systemOutputRunner(call)
				return err
			})
			for _, path := range inputs {
				if !filepath.IsLocal(path) {
					t.Fatalf("TOML input must be relative to its checkout: %q", path)
				}
			}
			if test.valid {
				if err != nil || len(output) != 0 {
					t.Fatalf("valid TOML checkout: %v\n%s", err, output)
				}
				return
			}
			if err == nil || !strings.Contains(filepath.ToSlash(string(output)), test.path) {
				t.Fatalf("TOML carrier %q was not diagnosed: %v\n%s", test.path, err, output)
			}
			if source := readFile(t, filepath.Join(root, filepath.FromSlash(test.path))); string(source) != test.content {
				t.Fatalf("check changed source: %q", source)
			}
		})
	}
}

func TestFormattingCoversCurrentCarriersAndPreservesOwnedExclusions(t *testing.T) {
	repository := repositoryRoot(t)
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"package.json", ".gitignore", ".editorconfig", ".prettierignore"} {
		if err := os.WriteFile(filepath.Join(root, path), readFile(t, filepath.Join(repository, path)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	files := []string{"document.md", "docs/archive/current.md", ".config/archive/metadata.json", "openspec/changes/archive/old/spec.md", "build/tracked.json", "literal[1].json", "unsupported.txt", "build/generated.json"}
	for _, path := range files {
		destination := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte("exact authored input\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "add", "-f", "build/tracked.json").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repository, ".git", "foreign-index"))
	called := false
	if err := run([]string{"check-format", root}, &bytes.Buffer{}, func(call command) error {
		called = true
		if call.Dir != root || call.Name != "node" || len(call.Args) != 1 || call.Args[0] != filepath.Join(root, "tools", "ci", "format.mjs") {
			t.Fatalf("native formatting invocation: %#v", call)
		}
		var inventory []string
		if err := json.Unmarshal([]byte(call.Input), &inventory); err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			if strings.Contains(call.Input, filepath.ToSlash(root)) {
				t.Fatal("format inventory must resolve relative to the requested checkout")
			}
			if slices.Contains(inventory, filepath.FromSlash(path)) == (path == "build/generated.json") {
				t.Fatalf("authored inventory membership for %s: %v", path, inventory)
			}
		}
		return nil
	}); err != nil || !called {
		t.Fatalf("format inventory not executed: %v", err)
	}
}
