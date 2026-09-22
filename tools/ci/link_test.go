package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLinksChecksCurrentRepositoryMarkdown(t *testing.T) {
	requireMiseTool(t, "github:lycheeverse/lychee")
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
		"--offline", "--include-fragments=anchor-only", "--no-progress", "--cache=false", "--files-from", "-",
	}, Input: strings.Join([]string{
		filepath.Join(root, "--literal.md"), filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "guide.md"), filepath.Join(root, "new-guide.md"),
	}, "\n") + "\n"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("link command = %#v, want %#v", got, want)
	}
}

func TestLinksChecksLocalHeadingTargets(t *testing.T) {
	requireMiseTool(t, "github:lycheeverse/lychee")
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	guide := filepath.Join(root, "guide.md")
	if err := os.WriteFile(guide, []byte("# Guide\n\n## Setup and verification\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "add", "guide.md").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
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

func TestLinksRequireTrackedTargets(t *testing.T) {
	requireMiseTool(t, "github:lycheeverse/lychee")
	for _, target := range []struct {
		name      string
		reference string
		tracked   bool
	}{
		{"untracked document", "guide.md", false},
		{"ignored artifact", "build/result.json", false},
		{"untracked directory", "drafts/", false},
		{"tracked document", "guide.md", true},
		{"tracked ignored artifact", "build/result.json", true},
		{"tracked directory", "guide/", true},
	} {
		t.Run(target.name, func(t *testing.T) {
			root := t.TempDir()
			git := func(args ...string) {
				t.Helper()
				if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, output)
				}
			}
			git("init", "--quiet")
			file := target.reference
			if strings.HasSuffix(file, "/") {
				file += "README.md"
			}
			write := func(name, content string) {
				t.Helper()
				filename := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write(file, "# Guide\n")
			write(".gitignore", "build/\n")
			write("README.md", "# Readme\n\n[Reference]("+target.reference+")\n")
			git("add", "README.md", ".gitignore")
			if target.tracked {
				git("add", "--force", file)
			}
			err := checkLinks(root, func(call command) error { _, err := systemOutputRunner(call); return err })
			if (err == nil) != target.tracked {
				t.Fatalf("tracked=%t error=%v", target.tracked, err)
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
