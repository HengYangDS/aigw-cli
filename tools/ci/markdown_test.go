package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownPolicyCommandUsesRequestedCheckout(t *testing.T) {
	if err := run([]string{"check-markdown-policy", repositoryRoot(t)}, &bytes.Buffer{}, systemRunner); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"check-markdown-policy", t.TempDir()}, &bytes.Buffer{}, systemRunner); err == nil {
		t.Fatal("Markdown policy command ignored its requested checkout")
	}
}

func TestDocumentInputsAreIndependentOfCommandLineLength(t *testing.T) {
	root := newGitRepository(t)
	var files []string
	for index := 0; len(strings.Join(files, "\n")) <= 32767; index++ {
		name := filepath.Join(root, fmt.Sprintf("%03d-document.md", index))
		if err := os.WriteFile(name, []byte("# Document\n\n[Heading](#document)\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, name)
	}
	if output, err := exec.Command("git", "-C", root, "add", "--all").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	for name, argumentCount := range map[string]int{"check-mermaid": 1, "links": 6} {
		t.Run(name, func(t *testing.T) {
			if name == "links" {
				requireMiseTool(t, "github:lycheeverse/lychee")
			}
			if err := run([]string{name, root}, &bytes.Buffer{}, func(call command) error {
				if len(call.Args) != argumentCount {
					t.Fatalf("document inventory escaped onto the command line: %d arguments", len(call.Args))
				}
				if name == "links" {
					if call.Input != strings.Join(files, "\n")+"\n" {
						t.Fatal("link input does not preserve the complete repository inventory")
					}
					if output, err := systemOutputRunner(call); err != nil {
						return fmt.Errorf("native link check: %w\n%s", err, output)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMarkdownCheckDiscoversAuthoredDocuments(t *testing.T) {
	repository := repositoryRoot(t)
	root := newGitRepository(t)
	git := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, output)
		}
	}
	valid, invalid := "# Document\n\n## Details\n", "# Document\n\n### Details\n"
	files := map[string]string{
		".gitignore":                           "/build/\n/node_modules/\n.serena/\n",
		"README.md":                            valid,
		"tools/domain/README.md":               valid,
		".config/domain/README.md":             valid,
		"docs/archive/current.md":              valid,
		"build/tracked.md":                     valid,
		"docs/[literal].md":                    valid,
		"docs/.markdownlint.json":              `{"MD001": false}`,
		"openspec/changes/archive/old/spec.md": invalid,
		"build/generated.md":                   invalid,
		"node_modules/dependency/README.md":    invalid,
		".serena/notes.md":                     invalid,
	}
	for _, relative := range []string{
		".config/checks/markdown/policy.yaml",
		"node_modules/markdownlint/schema/markdownlint-config-schema-strict.json",
	} {
		files[relative] = string(readFile(t, filepath.Join(repository, relative)))
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
	git("add", "--force", "build/tracked.md")
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repository, ".git", "not-the-requested-index"))
	for _, path := range []string{"README.md", "tools/domain/README.md", ".config/domain/README.md", "docs/archive/current.md", "build/tracked.md", "docs/override.md", "docs/[literal].md"} {
		t.Run(path, func(t *testing.T) {
			destination := filepath.Join(root, filepath.FromSlash(path))
			if err := os.WriteFile(destination, []byte(invalid), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.WriteFile(destination, []byte(valid), 0o600); err != nil {
					t.Error(err)
				}
			})
			var output []byte
			err := run([]string{"check-markdown", root}, &bytes.Buffer{}, func(call command) error {
				if len(call.Args) != 1 {
					t.Fatalf("Markdown inventory escaped onto command line: %d", len(call.Args))
				}
				call.Args[0] = filepath.Join(repository, "tools", "ci", "markdown", "lint.mjs")
				var err error
				output, err = systemOutputRunner(call)
				return err
			})
			if err == nil || !bytes.Contains(output, []byte("MD001")) || !bytes.Contains(output, []byte(filepath.FromSlash(path))) {
				t.Fatalf("authored document %q was not diagnosed: %v\n%s", path, err, output)
			}
			if string(readFile(t, destination)) != invalid {
				t.Fatal("check changed source")
			}
		})
	}
	if err := run([]string{"check-markdown", root}, &bytes.Buffer{}, func(call command) error {
		if len(call.Args) != 1 {
			t.Fatalf("Markdown inventory escaped onto command line: %d", len(call.Args))
		}
		call.Args[0] = filepath.Join(repository, "tools", "ci", "markdown", "lint.mjs")
		return systemRunner(call)
	}); err != nil {
		t.Fatalf("valid authored scope with ignored invalid files: %v", err)
	}
	for _, path := range []string{"README.md", "tools/domain/README.md", ".config/domain/README.md", "docs/archive/current.md", "build/tracked.md", "docs/override.md", "docs/[literal].md"} {
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	called := false
	if err := run([]string{"check-markdown", root}, &bytes.Buffer{}, func(command) error { called = true; return nil }); err == nil || called || !strings.Contains(err.Error(), "no current Markdown") {
		t.Fatalf("empty authored scope: %v, called=%t", err, called)
	}
}
