package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/terminal"
)

// A configuration file whose path runs through another file is refused by
// client edit and client enable before anything changes, naming that file
// with "not a directory" and the fix there, not a check that cannot resolve it.
func TestATargetUnderAFileIsRefusedBeforeAnyChange(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	configPath := runtime.Config.Path()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(file, "config.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"enable", codex.ID, "--executable", executable, "--target", target},
		{"edit", codex.ID, "--executable", executable, "--target", target},
	} {
		problem := terminal.ProblemFor(executeAdapter(t, runtime, args...))
		if !strings.Contains(problem.Title, "not a directory") || !strings.Contains(problem.Title, target) || !strings.Contains(problem.Fix, target) || strings.Contains(problem.Fix, "aigw check") {
			t.Fatalf("%v = %+v", args, problem)
		}
	}
	if after, err := os.ReadFile(configPath); err != nil || string(after) != string(before) {
		t.Fatalf("a refusal changed the configuration: %v", err)
	}
}
