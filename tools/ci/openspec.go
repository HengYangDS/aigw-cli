package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		Args: []string{"--run", "spec:check"},
		Dir:  root,
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
	if len(report.ItemFindings) == 0 {
		if totals.Failed != 0 {
			return errors.New("OpenSpec validation reports failures without findings")
		}
		_, err := fmt.Fprintf(stdout, "OpenSpec: %d items, 0 findings\n", header.TotalItems)
		return err
	}

	findings := make([]string, 0, report.Report.ReturnedItems)
	for _, item := range report.ItemFindings {
		for _, issue := range item.Issues {
			findings = append(findings, fmt.Sprintf("%s %s [%s]: %s", item.ID, issue.Path, issue.Level, issue.Message))
		}
	}
	if len(findings) == 0 {
		return errors.New("unexpected OpenSpec validation report")
	}
	return fmt.Errorf("OpenSpec validation findings:\n%s", strings.Join(findings, "\n"))
}
