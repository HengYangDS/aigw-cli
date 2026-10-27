package construction

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The scanner exits 1 for findings, but a report that names none leaves that
// exit unexplained, so it cannot pass as nonblocking findings.
func TestDependencyExitWithoutAdvisoryFails(t *testing.T) {
	if os.Getenv("AIGW_TEST_SCANNER_TERMINAL") == "unexplained" {
		os.Exit(1)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, "-test.run=^TestDependencyExitWithoutAdvisoryFails$")
	command.Env = append(os.Environ(), "AIGW_TEST_SCANNER_TERMINAL=unexplained")
	nativeExit := command.Run()
	if exit, ok := errors.AsType[*exec.ExitError](nativeExit); !ok || exit.ExitCode() != 1 {
		t.Fatalf("native scanner exit fixture = %v", nativeExit)
	}
	root := t.TempDir()
	report := dependencyReportFixture(root)
	for result := range report.Results {
		for item := range report.Results[result].Packages {
			report.Results[result].Packages[item].Vulnerabilities = nil
		}
	}
	_, err = scanDependencies(root, filepath.Join(root, "evidence"), scannedInputs, withLockedScanner(func(call toolCall) error {
		if err := writeJSON(call.Args[len(call.Args)-1], report); err != nil {
			return err
		}
		return fmt.Errorf("native findings: %w", nativeExit)
	}))
	if err == nil || !strings.Contains(err.Error(), "no complete advisory findings") {
		t.Fatalf("an exit without an advisory passed the dependency gate: %v", err)
	}
}
