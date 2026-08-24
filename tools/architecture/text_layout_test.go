package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTextLayoutAcceptsReadableTrackedFiles(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n\n```text\n\n\nLiteral spacing.\n```\n")
	writeFile(t, filepath.Join(root, "config.toml"), "version = 1\n\n# Profile.\n[profile]\nname = \"default\"\n")
	writeFile(t, filepath.Join(root, "fixture.bin"), "\x00\x01")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkTextLayout(root, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestTextLayoutRejectsOnlyDeterministicByteDefects(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project \n\n\nParagraph.\n\n")
	writeFile(t, filepath.Join(root, "config.toml"), "version = 1\n[profile]\nname = \"default\"\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkTextLayout(root, &report); err != nil {
		t.Fatal(err)
	}
	if !hasRule(report, "text_trailing_whitespace") {
		t.Fatalf("missing text_trailing_whitespace in %+v", report.Findings)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings=%+v, want only the deterministic byte defect", report.Findings)
	}
}

func TestTextLayoutRejectsMalformedMarkdownTableStructure(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n\n| First | Second | Third |\n| --- | --- |\n| A | B | C |\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkTextLayout(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "markdown_table_structure", "README.md")
}

func TestMarkdownTableStructureAcceptsValidTablesAndIgnoresFences(t *testing.T) {
	report := newReport("policy", ".")
	checkTextFile("README.md", []byte(strings.Join([]string{
		"| First | Second |",
		"| --- | :---: |",
		"| A | B |",
		"",
		"```markdown",
		"| First | Second | Third |",
		"| --- | --- |",
		"```",
		"",
		"~~~markdown",
		"| First | Second | Third |",
		"| --- | --- |",
		"~~~",
		"",
	}, "\n")), &report)
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestMarkdownTableHelpersHandleEscapesAndRejectNonDelimiters(t *testing.T) {
	if got := markdownTableColumns(`| one \| literal | two |`); got != 2 {
		t.Fatalf("escaped-pipe columns = %d", got)
	}
	for _, line := range []string{"plain text", "| -- | --- |", "| --- | text |"} {
		if count, ok := markdownTableDelimiterColumns(line); ok || count != 0 {
			t.Fatalf("delimiter %q = %d,%v", line, count, ok)
		}
	}
}

func TestTextFileReportsLineEndingsAndMissingFinalNewline(t *testing.T) {
	report := newReport("policy", ".")
	checkTextFile("README.md", []byte("line\r\nnext"), &report)
	assertFinding(t, report.Findings, "text_line_ending", "README.md")
	assertFinding(t, report.Findings, "text_final_newline", "README.md")

	empty := newReport("policy", ".")
	checkTextFile("README.md", nil, &empty)
	if !empty.OK {
		t.Fatalf("empty file findings=%+v", empty.Findings)
	}
}
