package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/surface"
	"aigw-cli/internal/transaction"
)

func standaloneCodexTarget(path string) TargetRef {
	return TargetRef{
		SurfaceID:      string(surface.CodexCLIStandalone),
		Authority:      string(surface.AuthorityAIGW),
		ProjectionMode: ProjectionFullSelection,
		Path:           path,
	}
}

func TestReconcileConfigsRestoresRemovedFullSelectionTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := []byte("model_provider = \"native\"\nmodel = \"gpt-native\"\nuser_setting = true\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	target := standaloneCodexTarget(path)
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withUserEdit := strings.Replace(string(projected), codexBegin, "after_projection = true\n"+codexBegin, 1)
	if err := os.WriteFile(path, []byte(withUserEdit), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs([]TargetRef{target}, nil, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`model_provider = "native"`,
		`model = "gpt-native"`,
		"user_setting = true",
		"after_projection = true",
	} {
		if !strings.Contains(string(restored), want) {
			t.Fatalf("restored config lacks %q:\n%s", want, restored)
		}
	}
	if strings.Contains(string(restored), "AIGW managed") {
		t.Fatalf("restored config still contains AIGW projection:\n%s", restored)
	}
	if _, err := os.Stat(codexStatePath(path)); !os.IsNotExist(err) {
		t.Fatalf("state remains after restore: %v", err)
	}
}

func TestReconcileConfigsRollsBackMixedRestoreAndAdd(t *testing.T) {
	dir := t.TempDir()
	restoredPath := filepath.Join(dir, "restore.toml")
	addedPath := filepath.Join(dir, "add.toml")
	for _, path := range []string{restoredPath, addedPath} {
		if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := []TargetRef{standaloneCodexTarget(restoredPath)}
	after := []TargetRef{standaloneCodexTarget(addedPath)}
	if _, err := ReconcileConfigs(nil, before, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		restoredPath,
		codexStatePath(restoredPath),
		addedPath,
		codexStatePath(addedPath),
	}
	snapshots := map[string]transaction.FileSnapshot{}
	for _, path := range paths {
		snapshot, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		snapshots[path] = snapshot
	}

	originalWrite := writeFileAtomicIfUnchanged
	originalRemove := removeFileIfUnchanged
	defer func() {
		writeFileAtomicIfUnchanged = originalWrite
		removeFileIfUnchanged = originalRemove
	}()
	writes := 0
	failOnFourth := func() error {
		writes++
		if writes == 4 {
			return errors.New("injected fourth artifact failure")
		}
		return nil
	}
	writeFileAtomicIfUnchanged = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if err := failOnFourth(); err != nil {
			return transaction.FileSnapshot{}, err
		}
		return originalWrite(path, expected, data, mode)
	}
	removeFileIfUnchanged = func(path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		if err := failOnFourth(); err != nil {
			return transaction.FileSnapshot{}, err
		}
		return originalRemove(path, expected)
	}

	_, err := ReconcileConfigs(before, after, atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "injected fourth artifact failure") {
		t.Fatalf("ReconcileConfigs() error = %v", err)
	}
	for _, path := range paths {
		got, readErr := transaction.CaptureFileSnapshot(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !reflect.DeepEqual(got, snapshots[path]) {
			t.Fatalf("%s after rollback = %#v, want %#v", path, got, snapshots[path])
		}
	}
}

func TestReconcileConfigsRejectsChangedPreimageWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalWrite := writeFileAtomicIfUnchanged
	defer func() { writeFileAtomicIfUnchanged = originalWrite }()
	mutated := false
	writeFileAtomicIfUnchanged = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if !mutated {
			mutated = true
			if err := os.WriteFile(path, []byte("newer external state\n"), 0o600); err != nil {
				return transaction.FileSnapshot{}, err
			}
		}
		return originalWrite(path, expected, data, mode)
	}
	_, err := ReconcileConfigs(nil, []TargetRef{standaloneCodexTarget(path)}, atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("ReconcileConfigs() error = %v, want preimage changed", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "newer external state\n" {
		t.Fatalf("config after rejected commit = %q, %v", got, readErr)
	}
}

func TestReconcileConfigsAdoptsOnlyCompleteAIGWSidecarAttribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := standaloneCodexTarget(path)
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	statePath := codexStatePath(path)
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}

	state.ProjectionMode = ""
	state.WriterID = ""
	state.TransactionID = ""
	legacy, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, append(legacy, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime()); err != nil {
		t.Fatalf("legacy full-selection adoption failed: %v", err)
	}
	adopted, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(adopted, &state); err != nil {
		t.Fatal(err)
	}
	if state.ProjectionMode != ProjectionFullSelection || state.WriterID != ProjectionWriterID || state.TransactionID == "" {
		t.Fatalf("legacy sidecar was not attributed: %#v", state)
	}

	for _, mutate := range []func(*codexState){
		func(state *codexState) { state.TransactionID = "" },
		func(state *codexState) { state.WriterID = "foreign-projector" },
	} {
		beforeConfig, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		beforeState, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(beforeState, &state); err != nil {
			t.Fatal(err)
		}
		mutate(&state)
		mutatedState, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(statePath, append(mutatedState, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		currentState, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime())
		if err == nil {
			t.Fatal("ReconcileConfigs() succeeded for invalid attribution")
		}
		afterConfig, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(afterConfig, beforeConfig) {
			t.Fatalf("config changed after attribution rejection: %q, %v", afterConfig, readErr)
		}
		afterState, readErr := os.ReadFile(statePath)
		if readErr != nil || !bytes.Equal(afterState, currentState) {
			t.Fatalf("state changed after attribution rejection: %q, %v", afterState, readErr)
		}
	}
}

func TestReconcileConfigsUsesLegacySidecarBesideSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(realDir, "configuration.toml")
	aliasPath := filepath.Join(dir, "alias.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-native\"\n"
	if err := os.WriteFile(realPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, aliasPath); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	if err := os.WriteFile(realPath, []byte(projectCodex(original, block, runtime.Model)), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := codexState{
		OriginalProvider: `model_provider = "native"`,
		OriginalModel:    `model = "gpt-native"`,
		ManagedBlockHash: hashText(block),
	}
	legacyData, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(aliasPath), append(legacyData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReconcileConfigs(nil, []TargetRef{standaloneCodexTarget(aliasPath)}, runtime); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(data), "[model_providers.aigw]"); count != 1 {
		t.Fatalf("managed provider tables = %d, want one:\n%s", count, data)
	}
	if _, err := os.Stat(codexStatePath(realPath)); !os.IsNotExist(err) {
		t.Fatalf("unexpected canonical sidecar created beside real target: %v", err)
	}
	stateData, err := os.ReadFile(codexStatePath(aliasPath))
	if err != nil {
		t.Fatal(err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}
	if state.WriterID != ProjectionWriterID || state.ProjectionMode != ProjectionFullSelection || state.TransactionID == "" {
		t.Fatalf("legacy symlink sidecar was not adopted: %#v", state)
	}
}

func TestValidateConfigRejectsForeignSidecarAttribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	if err := SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	stateData, err := os.ReadFile(codexStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}
	state.WriterID = "foreign-projector"
	mutated, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(path), append(mutated, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "foreign writer") {
		t.Fatalf("ValidateConfig() error = %v, want foreign writer", err)
	}
}
