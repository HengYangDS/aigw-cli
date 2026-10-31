//go:build performance_acceptance

package main

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/domain"

	"aigw-cli/tools/release/performance"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func (j *journeyFixture) attributionCases(helper []string, shell, reader, scope string) []performance.Workload {
	startup := []string{shell, "-c", "exit 0"}
	if runtime.GOOS == "windows" {
		startup = []string{shell, "/d", "/c", "exit", "0"}
	}
	return []performance.Workload{
		{Name: "credential", Command: helper, Budget: 0.1},
		{Name: "shell-startup", Command: startup},
		{Name: "source-startup", Command: []string{j.source, "--version"}},
		{Name: "reader-startup", Command: []string{reader, "--version"}},
		{Name: "credential-direct", Command: []string{reader, "credential", claude.ID, scope}},
		j.performanceCases(helper, "", domain.Runtime{})[1],
		j.performanceCases(helper, "", domain.Runtime{})[3],
	}
}

func TestNativeAttributionKeepsTheProjectionWorkload(t *testing.T) {
	tool, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	server := newNativeJourneyServer(t)
	t.Setenv("AIGW_PERFORMANCE_ATTRIBUTION", "1")
	for _, backend := range []string{"env", "keyring"} {
		t.Run(backend, func(t *testing.T) {
			// Use synthetic ENV credentials and stop before native store operations.
			journey := nativePerformanceJourney(t, performance.Program{Variant: "candidate", Path: program}, program, "env", server.URL+"/v1")
			installed := journey.binary
			journey.binary = filepath.Join(journey.root, "missing-program")
			rows, err := journey.measurePerformance(tool, t.TempDir(), "candidate", backend, 1)
			journey.binary = installed
			if err == nil || len(rows) == 0 || rows[len(rows)-1].Case != "projection" || rows[len(rows)-1].Backend != backend {
				t.Fatalf("attribution omitted selected %s projection (rows=%d): %v", backend, len(rows), err)
			}
		})
	}
}

func TestNativeAttributionKeepsTheProjectedReaderBoundary(t *testing.T) {
	journey := &journeyFixture{source: "verified source", binary: "installed path"}
	got := journey.attributionCases([]string{"original shell helper"}, "selected shell", "copied reader", "exact-scope")
	want := []performance.Workload{
		{Name: "credential", Command: []string{"original shell helper"}, Budget: 0.1},
		{Name: "shell-startup", Command: []string{"selected shell", "/d", "/c", "exit", "0"}},
		{Name: "source-startup", Command: []string{journey.source, "--version"}},
		{Name: "reader-startup", Command: []string{"copied reader", "--version"}},
		{Name: "credential-direct", Command: []string{"copied reader", "credential", claude.ID, "exact-scope"}},
		journey.performanceCases([]string{"original shell helper"}, "", domain.Runtime{})[1],
		journey.performanceCases([]string{"original shell helper"}, "", domain.Runtime{})[3],
	}
	if runtime.GOOS != "windows" {
		want[1].Command = []string{"selected shell", "-c", "exit 0"}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attribution changed the helper executable or qualification scope: %#v", got)
	}
}
