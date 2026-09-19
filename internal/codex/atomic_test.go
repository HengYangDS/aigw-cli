package codex

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

func atomicTestRuntime() configuration.Runtime {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return configuration.Runtime{
		CredentialCommand: executable,
		ProfileID:         "gpt-5.6-terra",
		ProfileLabel:      "GPT-5.6 Terra",
		AccountID:         "gateway",
		Client:            configuration.ClientCodex,
		Endpoint:          "http://127.0.0.1:48721/v1",
		Model:             "gpt-5.6-terra",
	}
}

func TestCodexProviderOwnershipSurvivesNativeTableEditing(t *testing.T) {
	for _, mode := range []string{"table before closing comment", "comments removed", "quoted keys", "interleaved auth"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			runtime := atomicTestRuntime()
			if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := SyncConfig(path, runtime); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			foreign := "[mcp_servers.fixture]\ncommand = \"/fixture/server\"\n"
			text := strings.Replace(string(data), codexEnd, foreign+codexEnd, 1)
			switch mode {
			case "comments removed":
				text = strings.ReplaceAll(strings.ReplaceAll(text, codexBegin+"\n", ""), codexEnd+"\n", "")
			case "quoted keys":
				text = strings.ReplaceAll(text, "model_providers.aigw", "model_providers.'aigw'")
				text = strings.Replace(text, "wire_api = \"responses\"", "wire_api = 'responses'", 1)
			case "interleaved auth":
				text = strings.Replace(string(data), "[model_providers.aigw.auth]", foreign+"\n[model_providers.aigw.auth]", 1)
			}
			if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ValidateConfig(path, runtime); err != nil {
				t.Fatalf("unchanged provider rejected after native table edit: %v", err)
			}
			inspection, err := InspectConfig(path)
			if err != nil || !inspection.SidecarHashMatches {
				t.Fatalf("semantic ownership lost: %+v: %v", inspection, err)
			}
			if err := SyncConfig(path, runtime); err != nil {
				t.Fatal(err)
			}
			if err := DisableConfig(path); err != nil {
				t.Fatal(err)
			}
			restored, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(restored), foreign) || strings.Contains(string(restored), "model_providers.") {
				t.Fatalf("withdrawal changed foreign table or retained provider: %q: %v", restored, err)
			}
		})
	}
}

func TestCodexProviderOwnershipRejectsChangedValuesBeforeWriting(t *testing.T) {
	for _, change := range []struct{ before, after string }{
		{"wire_api = \"responses\"", "wire_api = \"chat\""},
		{"http://127.0.0.1:48721/v1", "https://other.test/v1"},
		{"command = ", "other_command = "},
		{codexEnd, "unknown = true\n" + codexEnd},
	} {
		t.Run(change.before, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			runtime := atomicTestRuntime()
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := SyncConfig(path, runtime); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			changed := strings.Replace(string(original), change.before, change.after, 1)
			if changed == string(original) {
				t.Fatal("test did not change the owned value")
			}
			if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, operation := range []func() error{
				func() error { return ValidateConfig(path, runtime) },
				func() error { return SyncConfig(path, runtime) },
				func() error { return DisableConfig(path) },
			} {
				if err := operation(); err == nil {
					t.Fatal("changed provider was admitted")
				}
				actual, err := os.ReadFile(path)
				if err != nil || string(actual) != changed {
					t.Fatalf("conflicting source changed: %v", err)
				}
			}
		})
	}
}

func TestReconcileConfigsRollsBackEveryTargetAndAbsentStateOnWriteFailure(t *testing.T) {
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

	_, err := ReconcileConfigs(nil, codexHomeTargets([]string{first, second}), atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "injected state-write failure") {
		t.Fatalf("ReconcileConfigs() error = %v", err)
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

func TestReconcileConfigsPreflightRejectsLaterConflictWithoutChangingEarlierTarget(t *testing.T) {
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
	if err := SyncConfig(second, runtime); err != nil {
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

	_, err = ReconcileConfigs(nil, codexHomeTargets([]string{first, second}), runtime)
	if err == nil || !strings.Contains(err.Error(), "provider block changed") {
		t.Fatalf("ReconcileConfigs() error = %v, want later target conflict", err)
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

func TestPlanReconciliationClassifiesInitialConvergedAndReformattedProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	targets := codexHomeTargets([]string{path})
	plans, err := PlanReconciliation(nil, targets, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Action != "initial-project" {
		t.Fatalf("initial plan = %#v", plans)
	}
	if _, err := ReconcileConfigs(nil, targets, runtime); err != nil {
		t.Fatal(err)
	}
	plans, err = PlanReconciliation(nil, targets, runtime)
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
	plans, err = PlanReconciliation(nil, targets, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Action != "update" {
		t.Fatalf("truncated plan = %#v", plans)
	}
}

func TestConvergedCodexReconciliationPreservesEveryOwnedFile(t *testing.T) {
	path := writeCodexTestConfig(t, "model_provider = \"native\"\nuser_setting = true\n")
	targets := codexHomeTargets([]string{path})
	runtime := atomicTestRuntime()
	if _, err := ReconcileConfigs(nil, targets, runtime); err != nil {
		t.Fatal(err)
	}
	before := map[string]transaction.FileSnapshot{}
	for _, file := range []string{path, codexStatePath(path), codexCatalogPath(path)} {
		snapshot, err := transaction.CaptureFileSnapshot(file)
		if err != nil {
			t.Fatal(err)
		}
		before[file] = snapshot
	}
	prepared, err := prepareCodexReconciliation(targets, targets, runtime)
	if err != nil || len(prepared) != 1 || prepared[0].plan.Action != "already-converged" || len(prepared[0].artifacts) != 0 {
		t.Fatalf("converged preparation = %#v, %v", prepared, err)
	}
	if _, err := ReconcileConfigs(targets, targets, runtime); err != nil {
		t.Fatal(err)
	}
	for file, want := range before {
		got, err := transaction.CaptureFileSnapshot(file)
		if err != nil || !got.Equal(want) {
			t.Fatalf("converged reconciliation changed %s: %v", file, err)
		}
	}
}

func TestReconcileConfigsRejectsUnattributedStateWithoutOriginalSelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	unattributed := "model = \"gpt-5.6-terra\" # managed by AIGW\n" +
		"model_provider = \"aigw\" # managed by AIGW\n\n" +
		"user_setting = true\n\n" +
		codexBegin + "\n" + block
	if err := os.WriteFile(path, []byte(unattributed), 0o600); err != nil {
		t.Fatal(err)
	}
	stateData := []byte("{\n  \"managed_block_hash\": \"" + hashText(block) + "\"\n}\n")
	if err := os.WriteFile(codexStatePath(path), stateData, 0o600); err != nil {
		t.Fatal(err)
	}

	beforeConfig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReconcileConfigs(nil, codexHomeTargets([]string{path}), runtime)
	if err == nil || !strings.Contains(err.Error(), "attribution is incomplete") {
		t.Fatalf("ReconcileConfigs() error = %v, want incomplete attribution", err)
	}
	afterConfig, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(afterConfig, beforeConfig) {
		t.Fatalf("config changed after unattributed sidecar rejection: %q, %v", afterConfig, readErr)
	}
}
