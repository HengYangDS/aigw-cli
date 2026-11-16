package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const openSpecAcknowledgement = `{"level":"INFO","path":"file","message":"skip_specs is set in .openspec.yaml: change declares no spec-level behavior changes, zero deltas accepted"}`

func writeOpenSpecFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

type openSpecAdmissionCase struct {
	name    string
	items   []string
	failed  int
	wantErr string
	wantOut string
}

const openSpecAcknowledged = " file [INFO]: skip_specs is set in .openspec.yaml: change declares no spec-level behavior changes, zero deltas accepted"

func openSpecItem(id, kind string, valid bool, issues ...string) string {
	return fmt.Sprintf(`{"id":%q,"type":%q,"valid":%t,"issues":[%s],"durationMs":24}`, id, kind, valid, strings.Join(issues, ","))
}

func acknowledgedOpenSpecChange(id string) string {
	return openSpecItem(id, "change", true, openSpecAcknowledgement)
}

// checkOpenSpecAdmission drives the gate with a report of five items for each
// case, in a checkout whose Changes carry the given metadata.
func checkOpenSpecAdmission(t *testing.T, metadata map[string]string, tests []openSpecAdmissionCase) {
	t.Helper()
	root := t.TempDir()
	for file, content := range metadata {
		writeOpenSpecFixture(t, root, "openspec/changes/"+file, content)
	}
	encodedRoot, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := fmt.Sprintf(`{
  "report":{"kind":"validation-findings","version":"1.0","scope":"all","returnedItems":%d,"totalItems":5},
  "itemFindings":[%s],
  "summary":{"totals":{"items":5,"passed":%d,"failed":%d}},
  "root":{"path":%s}
}`, len(test.items), strings.Join(test.items, ","), 5-test.failed, test.failed, encodedRoot)
			var output bytes.Buffer
			err := validateOpenSpecReport([]byte(report), root, &output)
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
			if output.String() != test.wantOut {
				t.Fatalf("output = %q, want %q", output.String(), test.wantOut)
			}
		})
	}
}

const declaredOpenSpecMetadata = "schema: spec-driven\ncreated: 2026-11-16\nskip_specs: true\n"

// TestOpenSpecGateAdmitsADeclaredSpecFreeChange covers the scenario "A Change
// declares that it changes no specification" line by line.
func TestOpenSpecGateAdmitsADeclaredSpecFreeChange(t *testing.T) {
	checkOpenSpecAdmission(t, map[string]string{
		"declared/.openspec.yaml":      declaredOpenSpecMetadata,
		"also-declared/.openspec.yaml": "skip_specs: true\nschema: spec-driven\n",
		"undeclared/.openspec.yaml":    "schema: spec-driven\ncreated: 2026-11-16\n",
	}, []openSpecAdmissionCase{
		{
			name:    "declared",
			items:   []string{acknowledgedOpenSpecChange("declared")},
			wantOut: "OpenSpec: 5 items, 0 findings; without a specification change: declared\n",
		},
		{
			name:    "every declared Change is named",
			items:   []string{acknowledgedOpenSpecChange("declared"), acknowledgedOpenSpecChange("also-declared")},
			wantOut: "OpenSpec: 5 items, 0 findings; without a specification change: declared, also-declared\n",
		},
		{
			name: "declared with another finding",
			items: []string{openSpecItem("declared", "change", true, openSpecAcknowledgement,
				`{"level":"WARNING","path":"tasks.md","message":"unnumbered task"}`)},
			wantErr: "reported findings: 2):\ndeclared" + openSpecAcknowledged + "\ndeclared tasks.md [WARNING]: unnumbered task",
		},
		{
			name:    "acknowledged without the declaration",
			items:   []string{acknowledgedOpenSpecChange("undeclared")},
			wantErr: "reported findings: 1):\nundeclared" + openSpecAcknowledged,
		},
	})
}

// The declaration is `skip_specs: true` in the metadata of that one active
// Change; nothing else that resembles it admits the acknowledgement.
func TestOpenSpecGateReadsOnlyTheChangesOwnDeclaration(t *testing.T) {
	checkOpenSpecAdmission(t, map[string]string{
		"declared/.openspec.yaml":  declaredOpenSpecMetadata,
		"refused/.openspec.yaml":   "schema: spec-driven\nskip_specs: false\n",
		"quoted/.openspec.yaml":    "schema: spec-driven\nskip_specs: \"true\"\n",
		"commented/.openspec.yaml": "schema: spec-driven\n# skip_specs: true\n",
		"malformed/.openspec.yaml": "skip_specs: true\n\t- broken\n",
		".openspec.yaml":           declaredOpenSpecMetadata,
		"archive/.openspec.yaml":   declaredOpenSpecMetadata,
	}, []openSpecAdmissionCase{
		{name: "no metadata", items: []string{acknowledgedOpenSpecChange("absent")}, wantErr: "absent" + openSpecAcknowledged},
		{name: "a refusal", items: []string{acknowledgedOpenSpecChange("refused")}, wantErr: "refused" + openSpecAcknowledged},
		{name: "a value that is not a boolean", items: []string{acknowledgedOpenSpecChange("quoted")}, wantErr: "quoted" + openSpecAcknowledged},
		{name: "a comment", items: []string{acknowledgedOpenSpecChange("commented")}, wantErr: "commented" + openSpecAcknowledged},
		{name: "unreadable metadata", items: []string{acknowledgedOpenSpecChange("malformed")}, wantErr: "malformed" + openSpecAcknowledged},
		{name: "the directory of Changes", items: []string{acknowledgedOpenSpecChange(".")}, wantErr: "." + openSpecAcknowledged},
		{name: "the archive", items: []string{acknowledgedOpenSpecChange("archive")}, wantErr: "archive" + openSpecAcknowledged},
		{name: "a path to a Change", items: []string{acknowledgedOpenSpecChange("declared/../declared")}, wantErr: "declared/../declared" + openSpecAcknowledged},
	})
}

// Every finding that is not the acknowledgement of a declared Change fails the
// gate as before, and the failure reports every finding.
func TestOpenSpecGateKeepsEveryOtherFindingOfADeclaredChange(t *testing.T) {
	reworded := func(old, replacement string) string {
		return openSpecItem("declared", "change", true, strings.Replace(openSpecAcknowledgement, old, replacement, 1))
	}
	checkOpenSpecAdmission(t, map[string]string{"declared/.openspec.yaml": declaredOpenSpecMetadata}, []openSpecAdmissionCase{
		{
			name:    "another information",
			items:   []string{openSpecItem("declared", "change", true, `{"level":"INFO","path":"file","message":"Requirement text is very long"}`)},
			wantErr: "declared file [INFO]: Requirement text is very long",
		},
		{
			name:    "the acknowledgement's words at another path",
			items:   []string{reworded(`"path":"file"`, `"path":"proposal.md"`)},
			wantErr: "declared proposal.md [INFO]: skip_specs is set in .openspec.yaml",
		},
		{
			name:    "the acknowledgement's words as a warning",
			items:   []string{reworded(`"level":"INFO"`, `"level":"WARNING"`)},
			wantErr: "declared file [WARNING]: skip_specs is set in .openspec.yaml",
		},
		{
			name:    "another message about the declaration",
			items:   []string{reworded("skip_specs is set in .openspec.yaml: change declares", "skip_specs is ignored: change declares")},
			wantErr: "declared file [INFO]: skip_specs is ignored",
		},
		{
			name:    "unknown severity",
			items:   []string{reworded(`"level":"INFO"`, `"level":"NOTE"`)},
			wantErr: `unknown OpenSpec validation severity "NOTE"`,
		},
		{
			name:    "a specification of that name",
			items:   []string{openSpecItem("declared", "spec", true, openSpecAcknowledgement)},
			wantErr: "declared" + openSpecAcknowledged,
		},
		{
			name:    "an invalid Change",
			items:   []string{openSpecItem("declared", "change", false, openSpecAcknowledgement)},
			wantErr: "declared" + openSpecAcknowledged,
		},
		{
			name:    "a failed summary beside the declaration",
			items:   []string{acknowledgedOpenSpecChange("declared")},
			failed:  1,
			wantErr: "OpenSpec validation findings (validator failures: 1, reported findings: 1):\ndeclared" + openSpecAcknowledged,
		},
		{
			name: "another item's finding beside the declaration",
			items: []string{acknowledgedOpenSpecChange("declared"),
				openSpecItem("product-quality", "spec", true, `{"level":"INFO","path":"requirements[0]","message":"too long"}`)},
			wantErr: "reported findings: 2):\ndeclared" + openSpecAcknowledged + "\nproduct-quality requirements[0] [INFO]: too long",
		},
	})
}

// TestOpenSpecGateReadsTheValidatorsOwnAcknowledgement drives the gate with the
// bundled validator's report, so that a reworded acknowledgement fails here
// and names its cause.
func TestOpenSpecGateReadsTheValidatorsOwnAcknowledgement(t *testing.T) {
	checker := filepath.Join(repositoryRoot(t), "node_modules", "@fission-ai", "openspec", "bin", "openspec.js")
	root := t.TempDir()
	writeOpenSpecFixture(t, root, "openspec/specs/example/spec.md", "# Example\n\n## Purpose\n\nDescribe one complete example capability for isolated repository validation.\n\n## Requirements\n\n### Requirement: Example behavior\n\nThe example SHALL complete.\n\n#### Scenario: Successful completion\n\n- **WHEN** requested\n- **THEN** completion is reported.\n")
	change := "openspec/changes/repair-example/"
	writeOpenSpecFixture(t, root, change+"proposal.md", "## Why\n\nThe example fails although its accepted scenario already states the behavior that it should have.\n\n## What Changes\n\n- The example completes as its accepted scenario states.\n")
	writeOpenSpecFixture(t, root, change+"tasks.md", "## 1. Example\n\n- [ ] 1.1 Complete the example.\n")
	const declared, undeclared = "schema: spec-driven\ncreated: 2026-11-16\nskip_specs: true\n", "schema: spec-driven\ncreated: 2026-11-16\n"
	const delta = "## ADDED Requirements\n\n### Requirement: Example repair\n\nThe example SHALL repair itself.\n\n#### Scenario: Successful repair\n\n- **WHEN** requested\n- **THEN** the repair is reported.\n"
	t.Chdir(root)
	for _, item := range []struct {
		name     string
		metadata string
		delta    string
		wantErr  string
		wantOut  string
	}{
		{"declared without a delta", declared, "", "", "OpenSpec: 2 items, 0 findings; without a specification change: repair-example\n"},
		{"undeclared without a delta", undeclared, "", "Change must have at least one delta", ""},
		{"declared with a delta", declared, delta, "skip_specs is set in .openspec.yaml but spec files exist", ""},
	} {
		t.Run(item.name, func(t *testing.T) {
			writeOpenSpecFixture(t, root, change+".openspec.yaml", item.metadata)
			if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(change+"specs"))); err != nil {
				t.Fatal(err)
			}
			if item.delta != "" {
				writeOpenSpecFixture(t, root, change+"specs/example/spec.md", item.delta)
			}
			var stdout bytes.Buffer
			err := runOpenSpecValidation(&stdout, func(call command) ([]byte, error) {
				return systemOutputRunner(command{Name: "node", Dir: call.Dir, Args: append([]string{checker}, call.Args[1:]...)})
			})
			if item.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if item.wantErr != "" && (err == nil || !strings.Contains(err.Error(), item.wantErr)) {
				t.Fatalf("error = %v, want containing %q", err, item.wantErr)
			}
			if stdout.String() != item.wantOut {
				t.Fatalf("output = %q, want %q", stdout.String(), item.wantOut)
			}
		})
	}
}
