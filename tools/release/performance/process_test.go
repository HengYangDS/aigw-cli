package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedIdentityMustBeComplete(t *testing.T) {
	complete := Identity{Path: "/owned/program", Format: "ELF", Machine: 62, Arch: "amd64", SHA256: strings.Repeat("a", 64), Bytes: 1}
	if err := reviewIdentity(&complete); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Identity){
		"path":   func(identity *Identity) { identity.Path = "" },
		"digest": func(identity *Identity) { identity.SHA256 = "a" },
		"size":   func(identity *Identity) { identity.Bytes = 0 },
		"arch":   func(identity *Identity) { identity.Arch = "" },
	} {
		changed := complete
		change(&changed)
		if err := reviewIdentity(&changed); err == nil {
			t.Fatalf("an identity without its %s was accepted", name)
		}
	}
	if err := reviewIdentity(nil); err == nil {
		t.Fatal("an absent identity was accepted")
	}
}

// The untimed preflight runs the preparation and the measured command once
// each and retains their streams; only the product's own diagnostics mark the
// workload unqualified, and they do not stop it.
func TestPreflightJudgesTheProductCommandOnce(t *testing.T) {
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
			run := []string{program, "-test.run=^TestPreflightJudgesTheProductCommandOnce$"}
			workload := Workload{Command: append(run, "measure"), Prepare: append(append([]string(nil), run...), "prepare")}
			if err := ObserveCommand(t.Context(), input, workload); err != nil {
				t.Fatalf("a product diagnostic stopped the workload instead of marking it: %v", err)
			}
			if got := preflightDiagnostics(output); got != test.diagnostic {
				t.Fatalf("preflight diagnostic verdict for %q = %t", test.marker, got)
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
