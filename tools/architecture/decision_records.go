package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var decisionRecordName = regexp.MustCompile(`^dr-([0-9]{4})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)

func checkDecisionRecords(root string, report *Report) error {
	directory := filepath.Join(root, "docs", "decisions")
	registerPath := filepath.Join(directory, "README.md")
	register, err := os.ReadFile(registerPath)
	if err != nil {
		report.addFinding(Finding{Rule: "decision_record_register_missing", Path: "docs/decisions/README.md", Message: "Decision Records require one canonical register"})
		return nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read Decision Records: %w", err)
	}
	sequences := map[int]bool{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "README.md" || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		relative := "docs/decisions/" + entry.Name()
		match := decisionRecordName.FindStringSubmatch(entry.Name())
		if match == nil {
			report.addFinding(Finding{Rule: "decision_record_name", Path: relative, Message: "Decision Record names must use dr-<four-digit-sequence>-<kebab-case-description>.md"})
			continue
		}
		sequence, _ := strconv.Atoi(match[1])
		if sequences[sequence] {
			report.addFinding(Finding{Rule: "decision_record_sequence_duplicate", Path: relative, Count: sequence, Message: "Decision Record sequence is already used"})
		}
		sequences[sequence] = true
		body, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
		if readErr != nil {
			return fmt.Errorf("read %s: %w", relative, readErr)
		}
		checkDecisionRecordBody(relative, sequence, string(body), report)
		registrations := strings.Count(string(register), "("+entry.Name()+")")
		if registrations == 0 {
			report.addFinding(Finding{Rule: "decision_record_unregistered", Path: relative, Message: "Decision Record is absent from the canonical register"})
		} else if registrations > 1 {
			report.addFinding(Finding{Rule: "decision_record_registration_duplicate", Path: relative, Count: registrations, Message: "Decision Record must appear exactly once in the canonical register"})
		}
	}
	return nil
}

func checkDecisionRecordBody(relative string, sequence int, body string, report *Report) {
	required := []string{"- Status: ", "- Date: ", "## Context", "## Decision", "## Consequences", "## Revisit Trigger"}
	if !strings.HasPrefix(body, "# DR-"+fourDigits(sequence)+": ") {
		report.addFinding(Finding{Rule: "decision_record_title", Path: relative, Message: "Decision Record title must match its sequence"})
	}
	for _, marker := range required {
		if !strings.Contains(body, marker) {
			report.addFinding(Finding{Rule: "decision_record_section_missing", Path: relative, Name: marker, Message: "Decision Record is missing required content"})
		}
	}
}

func fourDigits(value int) string {
	return fmt.Sprintf("%04d", value)
}
