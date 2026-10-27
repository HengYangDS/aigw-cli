//go:build windows

package performance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A product diagnostic in the observed preflight marks the workload, as on
// Unix, instead of stopping it: the execution evidence is kept, and the
// redacted streams are retained where Measure reads its verdict.
func TestWindowsPreflightDiagnosticMarksTheWorkloadWithoutStopping(t *testing.T) {
	emitWindowsExecutionEvidence()
	image := currentTestImage(t)
	tool, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	marker := filepath.Join(directory, "workload-executed")
	secret := "synthetic-native-evidence-value"
	input := Command{Tool: tool, Output: filepath.Join(directory, "observed.json"), Directory: directory,
		Environment: append(os.Environ(), "AIGW_TEST_NATIVE_EVIDENCE_SECRET="+secret, "AIGW_TEST_NATIVE_EVIDENCE_MARKER="+marker),
		Sensitive:   []string{secret}, Measurement: Measurement{Case: "status", Backend: "env", Executable: &image}}
	workload := Workload{Command: []string{image.Path, "-test.run=^TestWindowsPreflightDiagnosticMarksTheWorkloadWithoutStopping$", "diagnostic"}}
	rows, controllers, err := ObserveCommand(t.Context(), input, workload)
	if err != nil {
		t.Fatalf("a product diagnostic stopped the workload instead of marking it: %v", err)
	}
	if _, markerErr := os.Stat(marker); markerErr != nil || len(rows) != 1 || len(controllers) != 1 {
		t.Fatalf("marked workload lost its native evidence: rows=%v controllers=%v marker=%v", rows, controllers, markerErr)
	}
	assertWindowsExecutionsExited(t, append(controllers, rows...))
	if !preflightDiagnostics(input.Output) {
		t.Fatal("the retained preflight lost the product diagnostic")
	}
	for _, stream := range []string{"stdout", "stderr"} {
		data, readErr := os.ReadFile(preflightStream(input.Output, stream))
		if readErr != nil || len(data) == 0 || strings.Contains(string(data), secret) {
			t.Fatalf("marked native preflight did not retain redacted %s: %q, %v", stream, data, readErr)
		}
	}
}
