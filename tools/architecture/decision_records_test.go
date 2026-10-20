package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestDecisionRecordsReportMissingNavigationAndRegistration(t *testing.T) {
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
	assertFinding(t, report.Findings, "decision_record_unregistered", "docs/decisions/dr-0001-product-boundary.md")
}

// The shared ETHOS paradigm owns the rationale structure; this index check
// must not restate an older section list that the shared one replaced.
func TestDecisionRecordsLeaveRationaleStructureToTheSharedParadigm(t *testing.T) {
	root := t.TempDir()
	writeDecisionRecord(t, root, "dr-0001-product-boundary.md", 1)
	writeFile(t, filepath.Join(root, "docs", "decisions", decisionNavigationFile), "[DR-0001](dr-0001-product-boundary.md)\n")
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("a record in the shared structure was refused: %+v", report.Findings)
	}
}

func writeDecisionRecord(t *testing.T, root, name string, sequence int) {
	t.Helper()
	identity := fmt.Sprintf("DR-%04d", sequence)
	body := []byte("---\ndecision_id: " + identity + "\ndecision_status: accepted\nstate: canonical\n---\n\n# " + identity + ": Decision\n\n" +
		"## Context\n\nContext.\n\n## Decision\n\nDecision.\n\n## Alternatives Rejected\n\nAlternatives.\n\n" +
		"## Consequences and Boundary\n\nConsequences.\n\n## Evidence and Revisit\n\nEvidence.\n")
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
