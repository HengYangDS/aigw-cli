package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDocumentationNavigationAcceptsReachableCanonicalDocuments(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n\nSee [Documentation](docs/README.md).\n")
	writeFile(t, filepath.Join(root, "docs", "README.md"), "# Documentation\n\nSee [Guide](guide.md).\n")
	writeFile(t, filepath.Join(root, "docs", "guide.md"), "# Guide\n")
	runGit(t, root, "add", ".")

	report := newReport("policy", root)
	contract := policy{
		DocumentationRoots:   []string{"README.md", "docs"},
		DocumentationEntries: []string{"README.md", "docs/README.md"},
	}
	if err := checkDocumentationNavigation(root, contract, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestDocumentationNavigationRejectsOrphanCanonicalDocument(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n")
	writeFile(t, filepath.Join(root, "docs", "README.md"), "# Documentation\n")
	writeFile(t, filepath.Join(root, "docs", "orphan.md"), "# Orphan\n")
	runGit(t, root, "add", ".")

	report := newReport("policy", root)
	contract := policy{
		DocumentationRoots:   []string{"README.md", "docs"},
		DocumentationEntries: []string{"README.md", "docs/README.md"},
	}
	if err := checkDocumentationNavigation(root, contract, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "documentation_navigation", "docs/orphan.md")
}

func TestDocumentationNavigationFollowsAnchoredAndReferenceLinks(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n\n[Documentation][docs]\n\n[docs]: docs/README.md#start\n")
	writeFile(t, filepath.Join(root, "docs", "README.md"), "# Documentation\n")
	runGit(t, root, "add", ".")

	report := newReport("policy", root)
	contract := policy{
		DocumentationRoots:   []string{"README.md", "docs"},
		DocumentationEntries: []string{"README.md"},
	}
	if err := checkDocumentationNavigation(root, contract, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestDocumentationNavigationCoversImagesAndIgnoresForeignTargets(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), strings.Join([]string{
		"# Project",
		"",
		"![Guide](docs/guide.md)",
		"[Section](#local)",
		"[Web](https://example.com)",
		"[Mail](mailto:team@example.com)",
		"[Parent](../outside.md)",
		"[Text](docs/not-markdown.txt)",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(root, "docs", "guide.md"), "# Guide\n")
	runGit(t, root, "add", ".")

	report := newReport("policy", root)
	contract := policy{
		DocumentationRoots:   []string{"README.md", "docs"},
		DocumentationEntries: []string{"README.md"},
	}
	if err := checkDocumentationNavigation(root, contract, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestDocumentationNavigationHandlesEmptyAndInvalidRepositories(t *testing.T) {
	report := newReport("policy", ".")
	if err := checkDocumentationNavigation(t.TempDir(), policy{}, &report); err != nil {
		t.Fatal(err)
	}

	empty := t.TempDir()
	runGit(t, empty, "init", "-q")
	if err := checkDocumentationNavigation(empty, policy{DocumentationRoots: []string{"docs"}}, &report); err != nil {
		t.Fatal(err)
	}

	invalid := t.TempDir()
	if err := os.Mkdir(filepath.Join(invalid, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkDocumentationNavigation(invalid, policy{DocumentationRoots: []string{"docs"}}, &report); err == nil || !strings.Contains(err.Error(), "list tracked files") {
		t.Fatalf("invalid repository error = %v", err)
	}
}

func TestDocumentationNavigationRejectsMissingEntrypointAndUnreadableDocument(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	contract := policy{DocumentationRoots: []string{"README.md"}, DocumentationEntries: []string{"docs/README.md"}}
	if err := checkDocumentationNavigation(root, contract, &report); err == nil || !strings.Contains(err.Error(), "not a canonical Markdown file") {
		t.Fatalf("missing entrypoint error = %v", err)
	}

	if _, err := localMarkdownLinks(filepath.Join(root, "missing.md"), "missing.md"); err == nil || !strings.Contains(err.Error(), "read documentation") {
		t.Fatalf("missing document error = %v", err)
	}
}

func TestCanonicalDocumentationUsesDeclaredFileAndDirectoryRoots(t *testing.T) {
	files := []string{"README.md", "CONTRIBUTING.md", "docs/guide.md", "docs.txt", "other.md"}
	want := map[string]bool{"README.md": true, "docs/guide.md": true}
	got := canonicalDocumentation(files, []string{"README.md", "docs"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical documents = %#v, want %#v", got, want)
	}
	got = canonicalDocumentation(files, []string{"docs/"})
	if !reflect.DeepEqual(got, map[string]bool{"docs/guide.md": true}) {
		t.Fatalf("slash-root documents = %#v", got)
	}
}

func TestResolveLocalMarkdownTargetNormalizesOnlyMarkdownPaths(t *testing.T) {
	tests := []struct {
		target string
		want   string
		ok     bool
	}{
		{target: "guide.md#section", want: "docs/guide.md", ok: true},
		{target: "<guide.md>", want: "docs/guide.md", ok: true},
		{target: "guide.md title", want: "docs/guide.md", ok: true},
		{target: ""},
		{target: "#section"},
		{target: "https://example.com/guide.md"},
		{target: "mailto:team@example.com"},
		{target: "guide.md#", want: "docs/guide.md", ok: true},
		{target: "../.."},
		{target: "../../outside.md"},
		{target: "guide.txt"},
	}
	for _, test := range tests {
		got, ok := resolveLocalMarkdownTarget("docs/README.md", test.target)
		if got != test.want || ok != test.ok {
			t.Fatalf("target %q = %q,%v; want %q,%v", test.target, got, ok, test.want, test.ok)
		}
	}
}
