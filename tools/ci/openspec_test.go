package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOpenSpecValidationRequiresCompleteCleanEvidence(t *testing.T) {
	root := t.TempDir()
	encodedRoot, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	clean := fmt.Sprintf(`{
  "report":{"kind":"validation-findings","version":"1.0","scope":"all","returnedItems":0,"totalItems":1},
  "root":{"path":%s},
  "summary":{"totals":{"items":1,"passed":1,"failed":0}},
  "itemFindings":[]
}`, encodedRoot)
	withFinding := strings.Replace(clean, `"returnedItems":0`, `"returnedItems":1`, 1)
	tests := []struct {
		name    string
		report  string
		wantErr string
	}{
		{
			name:   "clean",
			report: clean,
		},
		{
			name:    "partial scope",
			report:  strings.Replace(clean, `"scope":"all"`, `"scope":"specs"`, 1),
			wantErr: "unexpected OpenSpec validation report",
		},
		{
			name:    "information",
			report:  strings.Replace(withFinding, `"itemFindings":[]`, `"itemFindings":[{"id":"product-quality","issues":[{"level":"INFO","path":"requirements[0]","message":"too long"}]}]`, 1),
			wantErr: "product-quality requirements[0] [INFO]: too long",
		},
		{
			name:    "malformed JSON",
			report:  `{`,
			wantErr: "decode OpenSpec validation report",
		},
		{
			name:    "invalid report",
			report:  strings.Replace(clean, `"kind":"validation-findings"`, `"kind":"unexpected"`, 1),
			wantErr: "unexpected OpenSpec validation report",
		},
		{
			name:    "inconsistent finding count",
			report:  withFinding,
			wantErr: "unexpected OpenSpec validation report",
		},
		{
			name:    "finding without an issue",
			report:  strings.Replace(withFinding, `"itemFindings":[]`, `"itemFindings":[{"id":"product-quality","issues":[]}]`, 1),
			wantErr: "unexpected OpenSpec validation report",
		},
		{
			name:    "inconsistent summary",
			report:  strings.Replace(clean, `"passed":1`, `"passed":0`, 1),
			wantErr: "unexpected OpenSpec validation report",
		},
		{
			name:    "failure without findings",
			report:  strings.Replace(clean, `"passed":1,"failed":0`, `"passed":0,"failed":1`, 1),
			wantErr: "OpenSpec validation reports failures without findings",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateOpenSpecReport([]byte(test.report), root, &bytes.Buffer{})
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestRunDispatchesOpenSpecValidation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	manifest := map[string]map[string]string{
		"scripts": {"spec:check": "node validator.mjs"},
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("package.json", body, 0o600); err != nil {
		t.Fatal(err)
	}
	script := `console.log(JSON.stringify({
  report: {kind: "validation-findings", version: "1.0", scope: "all", returnedItems: 0, totalItems: 10},
  itemFindings: [],
  summary: {totals: {items: 10, passed: 10, failed: 0}},
  root: {path: process.cwd()},
}));
process.exit(Number(process.env.AIGW_TEST_VALIDATOR_EXIT || 0));
`
	if err := os.WriteFile("validator.mjs", []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_TEST_VALIDATOR_EXIT", "0")
	var stdout bytes.Buffer
	if err := run([]string{"openspec"}, &stdout, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "0 findings") {
		t.Fatalf("validator output = %q", stdout.String())
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed-result")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"openspec"}, closed, nil); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("validation result output failure = %v", err)
	}
	t.Setenv("AIGW_TEST_VALIDATOR_EXIT", "7")
	if err := run([]string{"openspec"}, &bytes.Buffer{}, nil); err == nil {
		t.Fatal("valid-looking output concealed validator failure")
	}
}

func TestOpenSpecValidationMeasuresOnlyTheRequestedCheckout(t *testing.T) {
	repository := repositoryRoot(t)
	checker := filepath.Join(repository, "node_modules", "@fission-ai", "openspec", "bin", "openspec.js")
	root := t.TempDir()
	empty := t.TempDir()
	child := filepath.Join(root, "unrelated-checkout")
	spec := filepath.Join(root, "openspec", "specs", "example", "spec.md")
	for _, directory := range []string{filepath.Dir(spec), child, filepath.Join(empty, "openspec", "specs")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	content := "# Example\n\n## Purpose\n\nDescribe one complete example capability for isolated repository validation.\n\n## Requirements\n\n### Requirement: Example behavior\n\nThe example SHALL complete.\n\n#### Scenario: Successful completion\n\n- **WHEN** requested\n- **THEN** completion is reported.\n"
	if err := os.WriteFile(spec, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		cwd  string
		want string
	}{
		{"complete checkout", root, ""},
		{"parent fallback", child, "OpenSpec validation root"},
		{"empty checkout", empty, "OpenSpec validation checked no items"},
	} {
		t.Run(item.name, func(t *testing.T) {
			t.Chdir(item.cwd)
			var stdout bytes.Buffer
			err := runOpenSpecValidation(&stdout, func(call command) ([]byte, error) {
				if call.Name != "node" || !reflect.DeepEqual(call.Args, []string{"--run", "spec:check"}) || call.Dir != item.cwd {
					t.Fatalf("validation command = %#v", call)
				}
				return systemOutputRunner(command{Name: "node", Dir: call.Dir, Args: []string{checker, "validate", "--all", "--strict", "--report", "findings", "--json", "--no-interactive"}})
			})
			if item.want == "" {
				if err != nil || !strings.Contains(stdout.String(), "1 items, 0 findings") {
					t.Fatalf("complete native validation: %v, %s", err, stdout.String())
				}
			} else if err == nil || !strings.Contains(err.Error(), item.want) || stdout.Len() != 0 {
				t.Fatalf("scope admission: %v, stdout=%q; want %q", err, stdout.String(), item.want)
			}
		})
	}
}

func TestOpenSpecCommandReportsAnUnavailableValidator(t *testing.T) {
	err := runOpenSpecValidation(&bytes.Buffer{}, func(command) ([]byte, error) {
		return []byte("validator unavailable"), errors.New("not found")
	})
	if err == nil || !strings.Contains(err.Error(), "OpenSpec validation failed") {
		t.Fatalf("error = %v", err)
	}
}
