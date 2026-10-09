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
	writeFile(t, filepath.Join(directory, decisionRegister), "# Decisions\n")
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
	writeFile(t, filepath.Join(directory, decisionRegister), "# Decisions\n")
	want := errors.New("directory read failed")
	report := newReport("policy", root)
	err := checkDecisionRecordsWithReadDir(root, &report, func(string) ([]os.DirEntry, error) {
		return nil, want
	})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "read Decision Records") {
		t.Fatalf("directory read error = %v", err)
	}
}

func TestDecisionRegisterReadFailureIsNotReportedAsMissing(t *testing.T) {
	root := t.TempDir()
	registerPath := filepath.Join(root, "docs", "decisions", decisionRegister)
	if err := os.MkdirAll(registerPath, 0o700); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	err := checkDecisionRecords(root, &report)
	if err == nil || !strings.Contains(err.Error(), "read Decision Register") {
		t.Fatalf("Decision Register read error = %v", err)
	}
	if hasRule(report, "decision_record_register_missing") {
		t.Fatalf("unreadable Decision Register was classified as missing: %+v", report.Findings)
	}
}

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

func TestDecisionRecordsAcceptNativeMetadataAndDistinctNavigation(t *testing.T) {
	for _, carrier := range []string{"yaml", "comment", "comment-crlf"} {
		t.Run(carrier, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "docs", "decisions")
			name := "dr-0001-product-boundary.md"
			writeDecisionRecord(t, root, name, 1)
			body, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil {
				t.Fatal(err)
			}
			metadata := "---\nsubject: example:decision:0001\nrole: decision\nstate: canonical\nrelations: {}\n---\n"
			if carrier != "yaml" {
				metadata = "<!--\n" + metadata + "-->\n"
			}
			record := metadata + "\n" + string(body)
			if carrier == "comment-crlf" {
				record = strings.ReplaceAll(record, "\n", "\r\n")
			}
			writeFile(t, filepath.Join(directory, name), record)
			writeFile(t, filepath.Join(directory, decisionRegister), "[Decision]("+name+")\n")
			writeFile(t, filepath.Join(directory, "README.md"), "---\nrole: index\n---\n\n# Decisions\n\n[Register](decision-register.md).\n")
			writeFile(t, filepath.Join(directory, "rationale.md"), "---\nrole: explanation\n---\n\n# Rationale\n\nRead the register.\n")
			report := newReport("policy", root)
			if err := checkDecisionRecords(root, &report); err != nil {
				t.Fatal(err)
			}
			if !report.OK {
				t.Fatalf("native document carriers rejected: %+v", report.Findings)
			}
		})
	}
}

func TestDecisionRecordsRequireRegisteredDecisionIdentityAfterMetadata(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "decisions")
	writeFile(t, filepath.Join(directory, decisionRegister), "# Register\n")
	writeFile(t, filepath.Join(directory, "boundary.md"), "---\nrole: decision\n---\n\n# DR-0001: Boundary\n")
	writeFile(t, filepath.Join(directory, "dr-0002-wrong-title.md"), "<!--\n---\nrole: decision\n---\n-->\n\n# Wrong title\n\n# DR-0002: Hidden later\n")
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report.Findings, "decision_record_name", "docs/decisions/boundary.md")
	assertFinding(t, report.Findings, "decision_record_title", "docs/decisions/dr-0002-wrong-title.md")
	assertFinding(t, report.Findings, "decision_record_unregistered", "docs/decisions/dr-0002-wrong-title.md")
}

func TestDecisionRecordMetadataFailureIsNotSilentlySkipped(t *testing.T) {
	for _, body := range []string{
		"---\nrole: decision\n",
		"<!--\n---\nrole: decision\n---\n# Missing comment close\n",
		"---\nrole: [\n---\n\n# DR-0001: Invalid YAML\n",
	} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "docs", "decisions")
			writeFile(t, filepath.Join(directory, decisionRegister), "# Register\n")
			writeFile(t, filepath.Join(directory, "dr-0001-invalid.md"), body)
			report := newReport("policy", root)
			if err := checkDecisionRecords(root, &report); err == nil || !strings.Contains(err.Error(), "read docs/decisions/dr-0001-invalid.md metadata") {
				t.Fatalf("metadata error = %v", err)
			}
		})
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
