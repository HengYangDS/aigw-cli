package upgrade

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"aigw-cli/internal/process"
)

func TestRollbackRetryCapabilityExitFixture(t *testing.T) {
	if os.Getenv("AIGW_TEST_RETRY_CAPABILITY_EXIT") == "1" {
		os.Exit(2)
	}
}

func TestRollbackRetryCapabilityRefusalPreservesPrograms(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), program, "-test.run=^TestRollbackRetryCapabilityExitFixture$")
	cmd.Env = []string{"AIGW_TEST_RETRY_CAPABILITY_EXIT=1"}
	if root := os.Getenv("SystemRoot"); root != "" {
		cmd.Env = append(cmd.Env, "SystemRoot="+root)
	}
	exit := cmd.Run()
	if _, ok := errors.AsType[*exec.ExitError](exit); !ok {
		t.Fatalf("fixture did not produce a real reader refusal: %v", exit)
	}
	root := t.TempDir()
	current := filepath.Join(root, "aigw")
	for path, content := range map[string]string{current: "current", RollbackPath(current): "retained"} {
		if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runner := &recordingRunner{inspect: func(plan process.Plan) ([]byte, error) {
		if slices.Equal(plan.Args, []string{"--version"}) {
			return []byte("aigw version 0.4.16\n"), nil
		}
		return nil, exit
	}}
	_, err = (Updater{Executable: current, Runner: runner}).Rollback(t.Context(), []byte("version = 6\n[clients.codex.retry_policy]\nrequest_max_retries = 0\n"), nil)
	if !errors.Is(err, ErrRollbackConfiguration) || !errors.Is(err, exit) {
		t.Fatalf("reader's policy refusal lost its semantic category: %v", err)
	}
	for path, want := range map[string]string{current: "current", RollbackPath(current): "retained"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("refused rollback changed %s: %v", path, err)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 2 {
		t.Fatalf("refused rollback retained staging: %v (%v)", entries, err)
	}
}
