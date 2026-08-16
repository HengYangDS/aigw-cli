package main

import (
	"path/filepath"
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
