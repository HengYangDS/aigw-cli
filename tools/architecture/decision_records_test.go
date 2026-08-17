package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecisionRecordsAcceptSemanticContiguousRegister(t *testing.T) {
	root := t.TempDir()
	writeDecisionRecord(t, root, "dr-0001-product-boundary.md", 1)
	writeDecisionRecord(t, root, "dr-0002-release-trust.md", 2)
	writeFile(t, filepath.Join(root, "docs", "decisions", decisionRegister), "[DR-0001](dr-0001-product-boundary.md)\n[DR-0002](dr-0002-release-trust.md)\n")
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
	writeFile(t, filepath.Join(root, "docs", "decisions", decisionRegister), "[DR-0002](dr-0002-release-trust.md)\n[again](dr-0002-release-trust.md)\n[DR-0004](dr-0004-portability.md)\n")
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

func TestDecisionRecordsReportMissingRegisterAndIncompleteBody(t *testing.T) {
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
	assertFinding(t, report.Findings, "decision_record_register_missing", "docs/decisions/decision-register.md")

	writeFile(t, filepath.Join(root, "docs", "decisions", decisionRegister), "# Decision Records\n")
	report = newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "decision_record_title", "docs/decisions/dr-0001-product-boundary.md")
	assertFinding(t, report.Findings, "decision_record_section_missing", "docs/decisions/dr-0001-product-boundary.md")
	assertFinding(t, report.Findings, "decision_record_unregistered", "docs/decisions/dr-0001-product-boundary.md")
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
