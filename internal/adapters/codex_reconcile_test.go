package adapters

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func standaloneCodexTarget(path string) CodexTargetRef {
	return CodexTargetRef{
		SurfaceID:      "codex-cli-standalone",
		Authority:      "aigw",
		ProjectionMode: CodexProjectionFullSelection,
		Path:           path,
	}
}

func airFallbackCodexTarget(path string) CodexTargetRef {
	return CodexTargetRef{
		SurfaceID:      "jetbrains-air-codex",
		Authority:      "jetbrains-ai",
		ProjectionMode: CodexProjectionNamespacedFallback,
		Path:           path,
	}
}

func TestReconcileCodexConfigsRestoresRemovedFullSelectionTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("model_provider = \"native\"\nmodel = \"gpt-native\"\nuser_setting = true\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	target := standaloneCodexTarget(path)
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{target}, atomicTestRuntime()); err != nil {
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
	if _, err := ReconcileCodexConfigs([]CodexTargetRef{target}, nil, atomicTestRuntime()); err != nil {
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

func TestReconcileCodexConfigsStagesAndRestoresAirFallbackByteExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("model_provider = \"jetbrains\"\nmodel = \"jb-default\"\n\n[jetbrains]\nenabled = true\n\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	target := airFallbackCodexTarget(path)
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{target}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	staged, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(staged, original) {
		t.Fatalf("Air fallback rewrote original bytes:\nwant prefix %q\ngot %q", original, staged)
	}
	for _, forbidden := range []string{
		`model_provider = "aigw"`,
		`model_provider = "aigw_fallback"`,
	} {
		if strings.Contains(string(staged), forbidden) {
			t.Fatalf("Air fallback changed top-level selection with %q:\n%s", forbidden, staged)
		}
	}
	if _, err := ReconcileCodexConfigs([]CodexTargetRef{target}, nil, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("Air bytes after restore differ\nwant %q\ngot  %q", original, restored)
	}
	if _, err := os.Stat(codexStatePath(path)); !os.IsNotExist(err) {
		t.Fatalf("Air fallback state remains after restore: %v", err)
	}
}

func TestReconcileCodexConfigsRollsBackMixedRestoreAndAdd(t *testing.T) {
	dir := t.TempDir()
	restoredPath := filepath.Join(dir, "restore.toml")
	addedPath := filepath.Join(dir, "add.toml")
	for _, path := range []string{restoredPath, addedPath} {
		if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := []CodexTargetRef{standaloneCodexTarget(restoredPath)}
	after := []CodexTargetRef{standaloneCodexTarget(addedPath)}
	if _, err := ReconcileCodexConfigs(nil, before, atomicTestRuntime()); err != nil {
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

	_, err := ReconcileCodexConfigs(before, after, atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "injected fourth artifact failure") {
		t.Fatalf("ReconcileCodexConfigs() error = %v", err)
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

func TestReconcileCodexConfigsRejectsChangedPreimageWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
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
	_, err := ReconcileCodexConfigs(nil, []CodexTargetRef{standaloneCodexTarget(path)}, atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("ReconcileCodexConfigs() error = %v, want preimage changed", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "newer external state\n" {
		t.Fatalf("config after rejected commit = %q, %v", got, readErr)
	}
}

func TestReconcileCodexConfigsAdoptsOnlyCompleteAIGWSidecarAttribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := standaloneCodexTarget(path)
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{target}, atomicTestRuntime()); err != nil {
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
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{target}, atomicTestRuntime()); err != nil {
		t.Fatalf("legacy full-selection adoption failed: %v", err)
	}
	adopted, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(adopted, &state); err != nil {
		t.Fatal(err)
	}
	if state.ProjectionMode != CodexProjectionFullSelection || state.WriterID != CodexProjectionWriterID || state.TransactionID == "" {
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
		_, err = ReconcileCodexConfigs(nil, []CodexTargetRef{target}, atomicTestRuntime())
		if err == nil {
			t.Fatal("ReconcileCodexConfigs() succeeded for invalid attribution")
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
