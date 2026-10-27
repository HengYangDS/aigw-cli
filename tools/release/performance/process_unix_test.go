//go:build !windows

package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The untimed Unix preflight runs the preparation and the measured command
// once each and judges only their own diagnostics; it records no Windows-only
// execution evidence.
func TestUnixPreflightJudgesTheProductCommandOnce(t *testing.T) {
	if runs := os.Getenv("AIGW_TEST_PREFLIGHT_RUNS"); runs != "" {
		step := os.Args[len(os.Args)-1]
		file, err := os.OpenFile(runs, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintln(file, step)
			_ = file.Close()
		}
		if step == "measure" {
			_, _ = fmt.Fprintln(os.Stderr, os.Getenv("AIGW_TEST_PREFLIGHT_STDERR"))
		}
		os.Exit(0)
	}
	rows, err := ObserveCurrent()
	if err != nil || rows != nil {
		t.Fatalf("Unix controller acquired Windows-only observation: rows=%v error=%v", rows, err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		marker     string
		diagnostic bool
	}{
		{"Working: product output", false},
		{"Warning: product output", true},
	} {
		t.Run(test.marker, func(t *testing.T) {
			directory := t.TempDir()
			runs, output := filepath.Join(directory, "runs"), filepath.Join(directory, "samples.json")
			input := Command{Directory: directory, Output: output, Environment: append(os.Environ(),
				"AIGW_TEST_PREFLIGHT_RUNS="+runs, "AIGW_TEST_PREFLIGHT_STDERR="+test.marker)}
			run := []string{program, "-test.run=^TestUnixPreflightJudgesTheProductCommandOnce$"}
			workload := Workload{Command: append(run, "measure"), Prepare: append(append([]string(nil), run...), "prepare")}
			rows, controllers, err := ObserveCommand(t.Context(), input, workload)
			if (err != nil) != test.diagnostic || test.diagnostic && !strings.Contains(err.Error(), "native preflight emitted diagnostics") {
				t.Fatalf("preflight decision for %q: %v", test.marker, err)
			}
			if rows != nil || controllers != nil {
				t.Fatalf("Unix preflight acquired Windows-only execution evidence: %v %v", rows, controllers)
			}
			if data, err := os.ReadFile(runs); err != nil || string(data) != "prepare\nmeasure\n" {
				t.Fatalf("the preflight did not run the preparation and the measured command once each: %q, %v", data, err)
			}
			if stderr, err := os.ReadFile(strings.TrimSuffix(output, ".json") + ".preflight.stderr"); err != nil || !strings.Contains(string(stderr), test.marker) {
				t.Fatalf("the preflight did not retain the product's stream: %q, %v", stderr, err)
			}
		})
	}
}
