package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecisionRecordReadFailureIsReported(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "decisions")
	writeFile(t, filepath.Join(directory, decisionNavigationFile), "# Decisions\n")
	if err := os.Symlink(filepath.Join(root, "missing-record"), filepath.Join(directory, "dr-0001-missing.md")); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err == nil || !strings.Contains(err.Error(), "read docs/decisions/dr-0001-missing.md") {
		t.Fatalf("decision record read error = %v", err)
	}
}

func TestDecisionRecordDirectoryReadFailureIsReported(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "decisions")
	writeFile(t, filepath.Join(directory, decisionNavigationFile), "# Decisions\n")
	want := errors.New("directory read failed")
	report := newReport("policy", root)
	err := checkDecisionRecordsWithReadDir(root, &report, func(string) ([]os.DirEntry, error) {
		return nil, want
	})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "read Decision Records") {
		t.Fatalf("directory read error = %v", err)
	}
}

func TestDecisionNavigationReadFailureIsNotReportedAsMissing(t *testing.T) {
	root := t.TempDir()
	navigationPath := filepath.Join(root, "docs", "decisions", decisionNavigationFile)
	if err := os.MkdirAll(navigationPath, 0o700); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	err := checkDecisionRecords(root, &report)
	if err == nil || !strings.Contains(err.Error(), "read Decision Record navigation") {
		t.Fatalf("Decision Record navigation read error = %v", err)
	}
	if hasRule(report, "decision_record_navigation_missing") {
		t.Fatalf("unreadable Decision Record navigation was classified as missing: %+v", report.Findings)
	}
}

func TestDecisionRecordsAcceptReadmeNavigation(t *testing.T) {
	root := t.TempDir()
	writeDecisionRecord(t, root, "dr-0001-product-boundary.md", 1)
	writeDecisionRecord(t, root, "dr-0002-release-trust.md", 2)
	writeFile(t, filepath.Join(root, "docs", "decisions", "README.md"), "[DR-0001](dr-0001-product-boundary.md)\n[DR-0002](dr-0002-release-trust.md)\n")
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("findings=%+v", report.Findings)
	}
}

func TestDecisionRecordsRejectBareNumbersAndDuplicateRegistrationWithoutRequiringContiguousHistory(t *testing.T) {
	root := t.TempDir()
	writeDecisionRecord(t, root, "0001-product-boundary.md", 1)
	writeDecisionRecord(t, root, "dr-0002-release-trust.md", 2)
	writeDecisionRecord(t, root, "dr-0004-portability.md", 4)
	writeFile(t, filepath.Join(root, "docs", "decisions", decisionNavigationFile), "[DR-0002](dr-0002-release-trust.md)\n[again](dr-0002-release-trust.md)\n[DR-0004](dr-0004-portability.md)\n")
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "decision_record_name", "docs/decisions/0001-product-boundary.md")
	assertFinding(t, report.Findings, "decision_record_registration_duplicate", "docs/decisions/dr-0002-release-trust.md")
	if countRule(report, "decision_record_sequence_gap") != 0 {
		t.Fatalf("historical numbering gap was treated as a product defect: %+v", report.Findings)
	}
}

func TestDecisionRecordsReportMissingNavigationAndIncompleteBody(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docs", "decisions", "dr-0001-product-boundary.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Wrong title\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "decision_record_navigation_missing", "docs/decisions/README.md")

	writeFile(t, filepath.Join(root, "docs", "decisions", decisionNavigationFile), "# Decision Records\n")
	report = newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "decision_record_title", "docs/decisions/dr-0001-product-boundary.md")
	assertFinding(t, report.Findings, "decision_record_section_missing", "docs/decisions/dr-0001-product-boundary.md")
	assertFinding(t, report.Findings, "decision_record_unregistered", "docs/decisions/dr-0001-product-boundary.md")
}

func TestDecisionRecordBodyChecksVisibleContentAfterMetadata(t *testing.T) {
	const visible = "# DR-0001: Decision\n\n- Status: accepted\n- Date: 2026-08-07\n\n## Context\nContext.\n\n## Decision\nDecision.\n\n## Consequences\nConsequences.\n\n## Revisit Trigger\nTrigger.\n"
	const metadata = "---\nsubject: aigw:decision:0001\nrole: decision\nstate: canonical\nrelations: {}\n---\n"
	tests := []struct {
		name                string
		body                string
		wantTitleFinding    bool
		wantSectionFindings bool
	}{
		{name: "plain frontmatter", body: metadata + "\n" + visible},
		{name: "title-first hidden frontmatter", body: "<!--\n" + metadata + "-->\n\n" + visible},
		{name: "CRLF hidden frontmatter", body: strings.ReplaceAll("<!--\n"+metadata+"-->\n\n"+visible, "\n", "\r\n")},
		{
			name:             "metadata cannot impersonate visible content",
			body:             "<!--\n---\nsubject: aigw:decision:0001\nrole: decision\nstate: canonical\nrelations: {}\nnotes: |\n  # DR-0001: Decision\n  - Status: accepted\n  - Date: 2026-08-07\n  ## Context\n  ## Decision\n  ## Consequences\n  ## Revisit Trigger\n---\n-->\n\n# Wrong visible title\n",
			wantTitleFinding: true, wantSectionFindings: true,
		},
		{name: "unfinished metadata", body: "<!--\n" + metadata + visible, wantTitleFinding: true, wantSectionFindings: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := newReport("policy", t.TempDir())
			checkDecisionRecordBody("docs/decisions/dr-0001-decision.md", 1, test.body, &report)
			if got := hasRule(report, "decision_record_title"); got != test.wantTitleFinding {
				t.Fatalf("title finding = %t, want %t: %+v", got, test.wantTitleFinding, report.Findings)
			}
			if got := hasRule(report, "decision_record_section_missing"); got != test.wantSectionFindings {
				t.Fatalf("section finding = %t, want %t: %+v", got, test.wantSectionFindings, report.Findings)
			}
		})
	}
}

func writeDecisionRecord(t *testing.T, root, name string, sequence int) {
	t.Helper()
	body := []byte("# DR-" + fourDigits(sequence) + ": Decision\n\n- Status: accepted\n- Date: 2026-08-07\n\n## Context\n\nContext.\n\n## Decision\n\nDecision.\n\n## Consequences\n\nConsequences.\n\n## Revisit Trigger\n\nTrigger.\n")
	path := filepath.Join(root, "docs", "decisions", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFinding(t *testing.T, findings []Finding, rule, path string) {
	t.Helper()
	for _, finding := range findings {
		if finding.Rule == rule && finding.Path == path {
			return
		}
	}
	t.Fatalf("missing %s:%s in %+v", rule, path, findings)
}
