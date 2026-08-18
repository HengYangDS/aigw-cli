package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
)

func stubVerifier(t *testing.T, verification codex.ModelCatalogVerification, err error) {
	t.Helper()
	original := verifyModelCatalog
	verifyModelCatalog = func(string, string) (codex.ModelCatalogVerification, error) {
		return verification, err
	}
	t.Cleanup(func() { verifyModelCatalog = original })
}

func stubExecutableLookup(t *testing.T, path string, err error) {
	t.Helper()
	original := lookupExecutable
	lookupExecutable = func(string) (string, error) { return path, err }
	t.Cleanup(func() { lookupExecutable = original })
}

func TestRunRequiresAModel(t *testing.T) {
	out := &bytes.Buffer{}
	code, err := run(nil, out)
	if code != exitPrerequisiteMissing || err == nil || !strings.Contains(err.Error(), "-model is required") {
		t.Fatalf("run() = %d, %v", code, err)
	}
}

func TestRunRejectsUnknownFlags(t *testing.T) {
	out := &bytes.Buffer{}
	code, err := run([]string{"-nonsense"}, out)
	if code != exitPrerequisiteMissing || err == nil {
		t.Fatalf("run() = %d, %v", code, err)
	}
}

// TestRunSeparatesAMissingClientFromAFailedCheck is the reason the command has
// two failure codes: a machine without the client must not report a pass, and
// must not report a defect either.
func TestRunSeparatesAMissingClientFromAFailedCheck(t *testing.T) {
	out := &bytes.Buffer{}
	absent := filepath.Join(t.TempDir(), "absent")
	code, err := run([]string{"-model", "openai.gpt-5.6-sol", "-codex", absent}, out)
	if code != exitPrerequisiteMissing {
		t.Fatalf("run() = %d, want %d", code, exitPrerequisiteMissing)
	}
	if err == nil || !strings.Contains(err.Error(), "prerequisite") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestResolveExecutablePrefersTheNamedClient(t *testing.T) {
	named := filepath.Join(t.TempDir(), "codex")
	if err := writeExecutable(named); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveExecutable(named)
	if err != nil || resolved != named {
		t.Fatalf("resolveExecutable() = %q, %v", resolved, err)
	}
}

func TestResolveExecutableUsesPATH(t *testing.T) {
	stubExecutableLookup(t, "/managed/codex", nil)
	resolved, err := resolveExecutable("")
	if err != nil || resolved != "/managed/codex" {
		t.Fatalf("resolveExecutable() = %q, %v", resolved, err)
	}
}

func TestResolveExecutableReportsMissingPATHClient(t *testing.T) {
	stubExecutableLookup(t, "", errors.New("missing"))
	_, err := resolveExecutable("")
	if err == nil || !strings.Contains(err.Error(), "no Codex executable") {
		t.Fatalf("resolveExecutable() error = %v", err)
	}
}

func TestRunReportsSuccessfulVerification(t *testing.T) {
	stubVerifier(t, sampleVerification(), nil)
	out := &bytes.Buffer{}
	code, err := run([]string{"-model", "openai.gpt-5.6-sol", "-codex", writeExecutableForTest(t)}, out)
	if code != 0 || err != nil || !strings.Contains(out.String(), "prefixed, generated catalog") {
		t.Fatalf("run() = %d, %v\n%s", code, err, out)
	}
}

func TestExecuteReportsTheFailure(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	code := execute(nil, out, errOut)
	if code != exitPrerequisiteMissing || !strings.Contains(errOut.String(), "-model is required") {
		t.Fatalf("execute() = %d\nstdout=%s\nstderr=%s", code, out, errOut)
	}
}

func TestRunSeparatesMeasurementAndVerdictFailures(t *testing.T) {
	client := writeExecutableForTest(t)
	for _, testCase := range []struct {
		name         string
		verification codex.ModelCatalogVerification
		verifyError  error
		wantError    string
	}{
		{"measurement", codex.ModelCatalogVerification{}, errors.New("probe failed"), "probe failed"},
		{"verdict", func() codex.ModelCatalogVerification {
			value := sampleVerification()
			value.Adapted = value.Unadapted
			return value
		}(), nil, "resolved"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stubVerifier(t, testCase.verification, testCase.verifyError)
			code, err := run([]string{"-model", "openai.gpt-5.6-sol", "-codex", client}, io.Discard)
			if code == 0 || err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("run() = %d, %v", code, err)
			}
		})
	}
}

func TestRunReportsPresentationFailures(t *testing.T) {
	stubVerifier(t, sampleVerification(), nil)
	original := writeReport
	writeReport = func(io.Writer, codex.ModelCatalogVerification, bool) error {
		return errors.New("render failed")
	}
	t.Cleanup(func() { writeReport = original })
	code, err := run(
		[]string{"-model", "openai.gpt-5.6-sol", "-codex", writeExecutableForTest(t)},
		io.Discard,
	)
	if code != exitVerificationFailed || err == nil || !strings.Contains(err.Error(), "render failed") {
		t.Fatalf("run() = %d, %v", code, err)
	}
}

func TestReportPropagatesWriterFailures(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		writer io.Writer
		json   bool
	}{
		{"json", failingWriter{}, true},
		{"heading", &failAtWriter{failAt: 1}, false},
		{"table flush", &failAtWriter{failAt: 2}, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if err := report(testCase.writer, sampleVerification(), testCase.json); err == nil {
				t.Fatalf("report() succeeded")
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

type failAtWriter struct {
	calls  int
	failAt int
}

func (writer *failAtWriter) Write(data []byte) (int, error) {
	writer.calls++
	if writer.calls == writer.failAt {
		return 0, errors.New("write failed")
	}
	return len(data), nil
}

func TestReportRendersEveryMeasuredSelection(t *testing.T) {
	verification := sampleVerification()
	out := &bytes.Buffer{}
	if err := report(out, verification, false); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"codex-cli 0.146.0",
		"openai.gpt-5.6-sol",
		"gpt-5.6-sol",
		"base slug, client's own table",
		"prefixed, no catalog",
		"prefixed, generated catalog",
		"unknown model, generated catalog",
		"17767",
		"20904",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report lacks %q:\n%s", want, text)
		}
	}
}

func TestReportEmitsMachineReadableMeasurements(t *testing.T) {
	out := &bytes.Buffer{}
	if err := report(out, sampleVerification(), true); err != nil {
		t.Fatal(err)
	}
	var decoded codex.ModelCatalogVerification
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("report emitted invalid JSON: %v\n%s", err, out)
	}
	if decoded != sampleVerification() {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func sampleVerification() codex.ModelCatalogVerification {
	resolved := codex.ModelCatalogProbe{Model: "gpt-5.6-sol", Instructions: 17767, Items: 5, MultiAgent: 2}
	fallback := codex.ModelCatalogProbe{Model: "openai.gpt-5.6-sol", Instructions: 20904, Items: 3}
	adapted := resolved
	adapted.Model = "openai.gpt-5.6-sol"
	unknown := fallback
	unknown.Model = "no-such-model"
	return codex.ModelCatalogVerification{
		ClientVersion: "codex-cli 0.146.0",
		ClientSHA256:  "134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477",
		Model:         "openai.gpt-5.6-sol",
		BaseSlug:      "gpt-5.6-sol",
		Reference:     resolved,
		Unadapted:     fallback,
		Adapted:       adapted,
		Unknown:       unknown,
	}
}

func writeExecutable(path string) error {
	return os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700)
}

func writeExecutableForTest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	if err := writeExecutable(path); err != nil {
		t.Fatal(err)
	}
	return path
}
