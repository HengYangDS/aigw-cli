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

const decisionNavigationFile = "README.md"

// checkDecisionRecords keeps the decision index complete: every record has a
// sequenced name and appears exactly once in the README navigation. The shared
// ETHOS paradigm owns each record's identity, disposition and rationale sections.
func checkDecisionRecords(root string, report *Report) error {
	return checkDecisionRecordsWithReadDir(root, report, os.ReadDir)
}

func checkDecisionRecordsWithReadDir(
	root string,
	report *Report,
	readDir func(string) ([]os.DirEntry, error),
) error {
	directory := filepath.Join(root, "docs", "decisions")
	navigationPath := filepath.Join(directory, decisionNavigationFile)
	navigation, err := os.ReadFile(navigationPath)
	if err != nil {
		if os.IsNotExist(err) {
			report.addFinding(Finding{Rule: "decision_record_navigation_missing", Path: "docs/decisions/" + decisionNavigationFile, Message: "Decision Records require README navigation"})
			return nil
		}
		return fmt.Errorf("read Decision Record navigation: %w", err)
	}
	entries, err := readDir(directory)
	if err != nil {
		return fmt.Errorf("read Decision Records: %w", err)
	}
	sequences := map[int]bool{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == decisionNavigationFile || filepath.Ext(entry.Name()) != ".md" {
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
		registrations := strings.Count(string(navigation), "("+entry.Name()+")")
		if registrations == 0 {
			report.addFinding(Finding{Rule: "decision_record_unregistered", Path: relative, Message: "Decision Record is absent from README navigation"})
		} else if registrations > 1 {
			report.addFinding(Finding{Rule: "decision_record_registration_duplicate", Path: relative, Count: registrations, Message: "Decision Record must appear exactly once in README navigation"})
		}
	}
	return nil
}
