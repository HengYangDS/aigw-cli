package adapters

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func atomicTestRuntime() domain.Runtime {
	return domain.Runtime{
		ProfileID:    "gpt-5.6-terra",
		ProfileLabel: "GPT-5.6 Terra",
		AccountID:    "gateway",
		Client:       domain.ClientCodex,
		Endpoint:     "http://127.0.0.1:8791/v1",
		Model:        "gpt-5.6-terra",
	}
}

func TestSyncCodexConfigsRollsBackEveryTargetAndAbsentStateOnWriteFailure(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.toml")
	second := filepath.Join(dir, "second.toml")
	firstBefore := []byte("model_provider = \"native\"\nfirst = true\n")
	secondBefore := []byte("model_provider = \"native\"\nsecond = true\n")
	for path, content := range map[string][]byte{first: firstBefore, second: secondBefore} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	originalWrite := writeFileAtomicIfUnchanged
	defer func() { writeFileAtomicIfUnchanged = originalWrite }()
	writes := 0
	writeFileAtomicIfUnchanged = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		writes++
		if writes == 4 { // second target state write, after the first target was fully committed
			return transaction.FileSnapshot{}, errors.New("injected state-write failure")
		}
		return originalWrite(path, expected, data, mode)
	}

	err := SyncCodexConfigs([]string{first, second}, atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "injected state-write failure") {
		t.Fatalf("SyncCodexConfigs() error = %v", err)
	}
	for path, want := range map[string][]byte{first: firstBefore, second: secondBefore} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != string(want) {
			t.Fatalf("%s after rollback = %q, %v; want %q", path, got, readErr, want)
		}
		if _, statErr := os.Stat(codexStatePath(path)); !os.IsNotExist(statErr) {
			t.Fatalf("state %s remains after rollback: %v", codexStatePath(path), statErr)
		}
	}
}

func TestSyncCodexConfigsPreflightRejectsLaterConflictWithoutChangingEarlierTarget(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.toml")
	second := filepath.Join(dir, "second.toml")
	firstBefore := "model_provider = \"native\"\nfirst = true\n"
	if err := os.WriteFile(first, []byte(firstBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	if err := SyncCodexConfig(second, runtime); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	conflicted := strings.Replace(string(projected), codexEnd+"\n", "foreign = \"do-not-overwrite\"\n", 1)
	if err := os.WriteFile(second, []byte(conflicted), 0o600); err != nil {
		t.Fatal(err)
	}

	err = SyncCodexConfigs([]string{first, second}, runtime)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("SyncCodexConfigs() error = %v, want later target conflict", err)
	}
	if got, err := os.ReadFile(first); err != nil || string(got) != firstBefore {
		t.Fatalf("earlier target changed during preflight: %q, %v", got, err)
	}
	if _, err := os.Stat(codexStatePath(first)); !os.IsNotExist(err) {
		t.Fatalf("earlier target state written during failed preflight: %v", err)
	}
	if got, err := os.ReadFile(second); err != nil || string(got) != conflicted {
		t.Fatalf("conflicting target changed during preflight: %q, %v", got, err)
	}
}

func TestPlanCodexConfigsClassifiesInitialConvergedAndExactTruncationRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	plans, err := PlanCodexConfigs([]string{path}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Action != "initial-project" {
		t.Fatalf("initial plan = %#v", plans)
	}
	if err := SyncCodexConfigs([]string{path}, runtime); err != nil {
		t.Fatal(err)
	}
	plans, err = PlanCodexConfigs([]string{path}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Action != "already-converged" {
		t.Fatalf("converged plan = %#v", plans)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	truncated := strings.Replace(string(projected), codexEnd+"\n", "", 1)
	if err := os.WriteFile(path, []byte(truncated), 0o600); err != nil {
		t.Fatal(err)
	}
	plans, err = PlanCodexConfigs([]string{path}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Action != "repair-truncated" {
		t.Fatalf("truncated plan = %#v", plans)
	}
}

func TestSyncCodexConfigsConvergesLegacyStateWithoutOriginalSelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime.ProfileLabel, runtime.Endpoint)
	legacy := "model = \"gpt-5.6-terra\" # managed by AIGW\n" +
		"model_provider = \"aigw\" # managed by AIGW\n\n" +
		"user_setting = true\n\n" +
		codexBegin + "\n" + block
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	stateData := []byte("{\n  \"managed_block_hash\": \"" + hashText(block) + "\"\n}\n")
	if err := os.WriteFile(codexStatePath(path), stateData, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SyncCodexConfigs([]string{path}, runtime); err != nil {
		t.Fatal(err)
	}
	plans, err := PlanCodexConfigs([]string{path}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Action != "already-converged" {
		t.Fatalf("plan after legacy-state convergence = %#v, want already-converged", plans)
	}
}
