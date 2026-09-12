package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

func TestLinksChecksCurrentRepositoryMarkdown(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		process := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := process.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet")
	write("README.md", "[valid](docs/guide.md)\n")
	write("--literal.md", "# Literal\n")
	write("docs/guide.md", "# Guide\n")
	write("new-guide.md", "# New guide\n")
	write("retired.md", "# Retired\n")
	write(".git/private.md", "[broken](missing.md)\n")
	git("add", "--", "README.md", "--literal.md", "docs/guide.md", "retired.md")
	if err := os.Remove(filepath.Join(root, "retired.md")); err != nil {
		t.Fatal(err)
	}

	var got command
	if err := run([]string{"links", root}, &bytes.Buffer{}, func(call command) error {
		got = call
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := command{Name: "lychee", Dir: root, Args: []string{
		"--offline", "--include-fragments=anchor-only", "--no-progress", "--cache=false", "--",
		filepath.Join(root, "--literal.md"), filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "guide.md"), filepath.Join(root, "new-guide.md"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("link command = %#v, want %#v", got, want)
	}
}

func TestLinksChecksLocalHeadingTargets(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	guide := filepath.Join(root, "guide.md")
	if err := os.WriteFile(guide, []byte("# Guide\n\n## Setup and verification\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		fragment string
		valid    bool
	}{
		{"setup-and-verification", true},
		{"nonexistent-heading", false},
	} {
		t.Run(test.fragment, func(t *testing.T) {
			source := "# Readme\n\n[Setup](guide.md#" + test.fragment + ")\n"
			readme := filepath.Join(root, "README.md")
			if err := os.WriteFile(readme, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			var output []byte
			err := run([]string{"links", root}, &bytes.Buffer{}, func(call command) error {
				var err error
				output, err = systemOutputRunner(call)
				return err
			})
			if (err == nil) != test.valid {
				t.Fatalf("heading target validity=%t: %v\n%s", test.valid, err, output)
			}
			if !test.valid && !bytes.Contains(output, []byte("Cannot find fragment")) {
				t.Fatalf("missing heading was not diagnosed: %s", output)
			}
			if got := string(readFile(t, readme)); got != source {
				t.Fatal("link check changed source")
			}
		})
	}
}

func TestLinksRejectsInvalidRepositoriesAndEmptyMarkdownSets(t *testing.T) {
	parent := t.TempDir()
	process := exec.Command("git", "-C", parent, "init", "--quiet")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	invalidRoot := filepath.Join(parent, "not-a-repository")
	if err := os.Mkdir(invalidRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := currentRepositoryFiles(invalidRoot, "Markdown", "*.md"); err == nil || !strings.Contains(err.Error(), "list repository Markdown") {
		t.Fatalf("non-repository error = %v", err)
	}
	if err := run([]string{"links", invalidRoot}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "list repository Markdown") {
		t.Fatalf("links non-repository error = %v", err)
	}

	root := t.TempDir()
	process = exec.Command("git", "-C", root, "init", "--quiet")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if markdown, err := currentRepositoryFiles(root, "Markdown", "*.md"); err == nil || !strings.Contains(err.Error(), "no current Markdown") || markdown != nil {
		t.Fatalf("empty Markdown set = %#v, error = %v", markdown, err)
	}
}

func TestLinksPropagatesLycheeFailure(t *testing.T) {
	root := t.TempDir()
	process := exec.Command("git", "-C", root, "init", "--quiet")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# AIGW\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	process = exec.Command("git", "-C", root, "add", "--", "README.md")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}

	want := errors.New("lychee failed")
	err := run([]string{"links", root}, &bytes.Buffer{}, func(command) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("link error = %v", err)
	}
}

func TestMarkdownPolicyEnforcesDocumentStructure(t *testing.T) {
	root := repositoryRoot(t)
	checker := filepath.Join(root, "tools", "ci", "markdown", "lint.mjs")
	policy := readFile(t, filepath.Join(root, ".config", "checks", "markdown", "policy.yaml"))
	for name, entry := range map[string]struct {
		content string
		rule    string
	}{
		"document":            {"# Document\n\n## First\n\n```sh\naigw status\n```\n", ""},
		"OpenSpec":            {"## ADDED Requirements\n\n### Requirement: First\n\n#### Scenario: Accepted\n\n- Expected.\n\n### Requirement: Second\n\n#### Scenario: Accepted\n\n- Expected.\n", ""},
		"heading progression": {"# Document\n\n### Details\n", "MD001"},
		"inline suppression":  {"<!-- markdownlint-disable MD001 -->\n# Document\n\n### Details\n", "MD001"},
		"sibling uniqueness":  {"# Document\n\n## Details\n\nText.\n\n## Details\n", "MD024"},
		"single title":        {"# First\n\nText.\n\n# Second\n", "MD025"},
		"heading punctuation": {"# Document\n\n## Details:\n", "MD026"},
		"question heading":    {"# Document\n\n## Which backend should I use?\n", ""},
		"semantic heading":    {"# Document\n\n**Details**\n\nText.\n", "MD036"},
		"emphasized sentence": {"# Document\n\n**Keep the original configuration.**\n", ""},
		"OpenSpec labels":     {"## Goals / Non-Goals\n\n**Goals:**\n\n- Preserve ownership.\n\n**Non-Goals:**\n\n- Change client state.\n", ""},
		"fence language":      {"# Document\n\n```\naigw status\n```\n", "MD040"},
		"separator style":     {"# Document\n\nText.\n\n***\n\nText.\n", "MD035"},
		"table columns":       {"# Document\n\n| First | Second |\n| ----- | ------ |\n| Value |\n", "MD056"},
		"table alignment":     {"# Document\n\n| First | Second |\n| ----- | ------ |\n| Value  | Result |\n", "MD060"},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := t.TempDir()
			if err := os.WriteFile(filepath.Join(fixture, "document.md"), []byte(entry.content), 0o600); err != nil {
				t.Fatal(err)
			}
			policyPath := filepath.Join(fixture, ".config", "checks", "markdown", "policy.yaml")
			if err := os.MkdirAll(filepath.Dir(policyPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(policyPath, policy, 0o600); err != nil {
				t.Fatal(err)
			}
			input, err := json.Marshal([]string{filepath.Join(fixture, "document.md")})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command("node", checker)
			command.Stdin = bytes.NewReader(input)
			command.Dir = fixture
			output, err := command.CombinedOutput()
			if entry.rule == "" {
				if err != nil || !bytes.Contains(output, []byte("checked 1 Markdown files")) {
					t.Fatalf("valid document: %v\n%s", err, output)
				}
				return
			}
			if err == nil || !bytes.Contains(output, []byte(entry.rule)) {
				t.Fatalf("expected %s from native document check: %v\n%s", entry.rule, err, output)
			}
		})
	}
}

func TestMermaidChecksExecuteAgainstRequestedSources(t *testing.T) {
	repository := repositoryRoot(t)
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	checker := filepath.Join("tools", "ci", "markdown", "diagrams.mjs")
	local := filepath.Join(root, checker)
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		t.Fatal(err)
	}
	entry, err := json.Marshal(filepath.Join(repository, checker))
	if err != nil {
		t.Fatal(err)
	}
	forward := "import { pathToFileURL } from 'node:url'; await import(pathToFileURL(" + string(entry) + ").href);\n"
	if err := os.WriteFile(local, []byte(forward), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repository, ".git", "nonexistent-index"))
	if err := os.WriteFile(filepath.Join(root, ".mermaidlintrc.json"), []byte(`{"ignore":["**/*"],"rules":{"no-self-loop":"off"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, diagram string
		valid         bool
	}{
		{"flowchart", "flowchart LR\n  A[Source] --> B[Target]\n", true},
		{"sequence", "sequenceDiagram\n  A->>B: Failure, no writes\n", true},
		{"semicolon message", "sequenceDiagram\n  A->>B: Failure; no writes\n", false},
		{"syntax", "flowchart LR\n  A[Unclosed\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, "diagram.md")
			body := []byte("# Diagram\n\n```mermaid\n" + test.diagram + "```\n")
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			var output []byte
			err := run([]string{"check-mermaid", root}, &bytes.Buffer{}, func(call command) error {
				var runErr error
				output, runErr = systemOutputRunner(call)
				return runErr
			})
			if (err == nil) != test.valid || !bytes.Contains(output, []byte("checked 1 diagram")) {
				t.Fatalf("diagram validity = %t, want %t: %v\n%s", err == nil, test.valid, err, output)
			}
			if !bytes.Equal(readFile(t, path), body) {
				t.Fatal("diagram check changed its source")
			}
		})
	}
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"check-mermaid", root}, &bytes.Buffer{}, systemRunner); err == nil || !strings.Contains(err.Error(), "MODULE_NOT_FOUND") {
		t.Fatalf("missing native checker error = %v", err)
	}
}

func TestMermaidInputIsIndependentOfCommandLineLength(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	for index := range 400 {
		name := filepath.Join(root, fmt.Sprintf("%03d-%s.md", index, strings.Repeat("document", 16)))
		if err := os.WriteFile(name, []byte("# Document\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := run([]string{"check-mermaid", root}, &bytes.Buffer{}, func(call command) error {
		if len(call.Args) != 1 {
			t.Fatalf("diagram inventory escaped onto the command line: %d arguments", len(call.Args))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownCheckDiscoversAuthoredDocuments(t *testing.T) {
	repository := repositoryRoot(t)
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, output)
		}
	}
	git("init", "--quiet")
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

func TestMarkdownAdapterRequiresNativeInputsAndRejectsWarnings(t *testing.T) {
	repository := repositoryRoot(t)
	checker := filepath.Join(repository, "tools", "ci", "markdown", "lint.mjs")
	for _, scenario := range []struct{ name, policy, diagnostic string }{
		{"warning", "MD001: warning\n", "MD001"},
		{"missing policy", "", "ENOENT"},
		{"missing dependency", "MD001: true\n", "ENOENT"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			policy := filepath.Join(root, ".config", "checks", "markdown", "policy.yaml")
			if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
				t.Fatal(err)
			}
			if scenario.policy != "" {
				if err := os.WriteFile(policy, []byte(scenario.policy), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(root, "document.md")
			content := []byte("# Document\n\n### Details\n")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			entry := checker
			if scenario.name == "missing dependency" {
				entry = filepath.Join(root, "tools", "ci", "markdown", "lint.mjs")
				if err := os.MkdirAll(filepath.Dir(entry), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(entry, readFile(t, checker), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			input, err := json.Marshal([]string{path})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command("node", entry)
			command.Dir, command.Stdin = root, bytes.NewReader(input)
			output, err := command.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte(scenario.diagnostic)) {
				t.Fatalf("%s: %v\n%s", scenario.name, err, output)
			}
			if !bytes.Equal(readFile(t, path), content) {
				t.Fatal("adapter changed input")
			}
		})
	}
}
