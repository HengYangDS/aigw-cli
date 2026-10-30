package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type openSpecValidationReport struct {
	Report struct {
		Kind          string `json:"kind"`
		Version       string `json:"version"`
		Scope         string `json:"scope"`
		ReturnedItems int    `json:"returnedItems"`
		TotalItems    int    `json:"totalItems"`
	} `json:"report"`
	Root struct {
		Path string `json:"path"`
	} `json:"root"`
	Summary struct {
		Totals struct {
			Items  int `json:"items"`
			Passed int `json:"passed"`
			Failed int `json:"failed"`
		} `json:"totals"`
	} `json:"summary"`
	ItemFindings []struct {
		ID     string `json:"id"`
		Issues []struct {
			Level   string `json:"level"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"issues"`
	} `json:"itemFindings"`
}

func runOpenSpecValidation(stdout io.Writer, runner outputRunner) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve OpenSpec validation checkout: %w", err)
	}
	output, err := runner(command{
		Name: "node",
		Args: []string{
			filepath.Join(root, "node_modules", "@fission-ai", "openspec", "bin", "openspec.js"),
			"validate", "--all", "--strict", "--report", "findings", "--json", "--no-interactive",
		},
		Dir: root,
	})
	if err != nil {
		return fmt.Errorf("OpenSpec validation failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return validateOpenSpecReport(output, root, stdout)
}

func validateOpenSpecReport(raw []byte, root string, stdout io.Writer) error {
	var report openSpecValidationReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("decode OpenSpec validation report: %w", err)
	}
	header, totals := report.Report, report.Summary.Totals
	if header.Kind != "validation-findings" || header.Version != "1.0" || header.Scope != "all" ||
		header.ReturnedItems != len(report.ItemFindings) || header.TotalItems < header.ReturnedItems ||
		totals.Items != header.TotalItems || totals.Passed < 0 || totals.Failed < 0 || totals.Passed+totals.Failed != totals.Items {
		return errors.New("unexpected OpenSpec validation report")
	}
	if header.TotalItems == 0 {
		return errors.New("OpenSpec validation checked no items")
	}
	expected, expectedErr := os.Stat(root)
	observed, observedErr := os.Stat(report.Root.Path)
	if !filepath.IsAbs(report.Root.Path) || expectedErr != nil || observedErr != nil || !observed.IsDir() || !os.SameFile(expected, observed) {
		return fmt.Errorf("OpenSpec validation root %q does not identify requested checkout %q", report.Root.Path, root)
	}
	findings := make([]string, 0, report.Report.ReturnedItems)
	failed := totals.Failed != 0 || header.ReturnedItems != 0
	for _, item := range report.ItemFindings {
		if len(item.Issues) == 0 {
			return errors.New("unexpected OpenSpec validation report")
		}
		for _, issue := range item.Issues {
			switch issue.Level {
			case "INFO", "WARNING", "ERROR":
			default:
				return fmt.Errorf("unknown OpenSpec validation severity %q", issue.Level)
			}
			findings = append(findings, fmt.Sprintf("%s %s [%s]: %s", item.ID, issue.Path, issue.Level, issue.Message))
		}
	}
	if failed {
		return fmt.Errorf("OpenSpec validation findings (validator failures: %d, reported findings: %d):\n%s", totals.Failed, len(findings), strings.Join(findings, "\n"))
	}
	summary := fmt.Sprintf("OpenSpec: %d items, %d findings", header.TotalItems, len(findings))
	_, err := fmt.Fprintln(stdout, strings.Join(append([]string{summary}, findings...), "\n"))
	return err
}

// openSpecTransition holds the active Changes and the main specifications whose
// requirements their deltas modify, remove or rename. Such a main specification
// changes only when its Change archives, so a gate reads the delta instead. A
// delta that only adds requirements leaves its main specification's text in
// place, and the gates read both.
type openSpecTransition struct {
	changes   []string
	rewritten map[string]bool
}

var rewritingDelta = regexp.MustCompile(`(?m)^## (?:MODIFIED|REMOVED|RENAMED) Requirements[ \t]*\r?$`)

// readOpenSpecTransition derives the active Changes and the main specifications
// that their deltas rewrite from files, the repository's current files as
// slash-separated paths relative to root.
func readOpenSpecTransition(root string, files []string) (openSpecTransition, error) {
	transition := openSpecTransition{rewritten: map[string]bool{}}
	changes := map[string]bool{}
	for _, file := range files {
		inChanges, ok := strings.CutPrefix(file, "openspec/changes/")
		if !ok {
			continue
		}
		change, record, found := strings.Cut(inChanges, "/")
		if !found || change == "archive" {
			continue
		}
		changes["openspec/changes/"+change] = true
		delta, ok := strings.CutPrefix(record, "specs/")
		capability, name, found := strings.Cut(delta, "/")
		if !ok || !found || name != "spec.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return transition, fmt.Errorf("read OpenSpec delta %s: %w", file, err)
		}
		if rewritingDelta.Match(data) {
			transition.rewritten["openspec/specs/"+capability+"/spec.md"] = true
		}
	}
	transition.changes = slices.Sorted(maps.Keys(changes))
	return transition, nil
}

// changeRecord reports whether file is one of an active Change's own records,
// such as its proposal, design or tasks, rather than one of its deltas.
func (transition openSpecTransition) changeRecord(file string) bool {
	for _, change := range transition.changes {
		if rest, ok := strings.CutPrefix(file, change+"/"); ok && !strings.Contains(rest, "/") {
			return true
		}
	}
	return false
}
