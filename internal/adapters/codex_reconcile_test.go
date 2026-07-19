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

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
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

func airStaleFullSelectionRecoveryTarget(path string) CodexTargetRef {
	return CodexTargetRef{
		SurfaceID:      "jetbrains-air-codex",
		Authority:      "jetbrains-ai",
		ProjectionMode: CodexProjectionStaleAirFullSelectionRecovery,
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

func TestReconcileCodexConfigsRecoversStaleAirFullSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"jetbrains\"\nmodel = \"jb-default\"\nuser_setting = true\n"
	runtime := atomicTestRuntime()
	fullBlock := codexManagedBlock(runtime.ProfileLabel, runtime.Endpoint)
	if err := os.WriteFile(path, []byte(projectCodex(original, fullBlock, runtime.Model)), 0o600); err != nil {
		t.Fatal(err)
	}
	fallbackBlock := codexFallbackBlock(runtime.ProfileLabel, runtime.Endpoint)
	staleState, err := json.Marshal(codexState{
		ManagedBlockHash: hashText(fallbackBlock),
		ProjectionMode:   CodexProjectionNamespacedFallback,
		WriterID:         CodexProjectionWriterID,
		TransactionID:    "stale-air-full-selection",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(path), append(staleState, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	target := airStaleFullSelectionRecoveryTarget(path)
	plans, err := PlanCodexReconciliation(nil, []CodexTargetRef{target}, domain.Runtime{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Action != "recover-stale-full-selection" {
		t.Fatalf("plans = %#v", plans)
	}
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{target}, domain.Runtime{}); err != nil {
		t.Fatal(err)
	}
	recovered, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`model_provider = "aigw"`,
		`# managed by AIGW`,
		"AIGW managed provider",
		"[model_providers.aigw]",
	} {
		if strings.Contains(string(recovered), forbidden) {
			t.Fatalf("recovered Air config retains %q:\n%s", forbidden, recovered)
		}
	}
	if !strings.Contains(string(recovered), "user_setting = true") {
		t.Fatalf("recovered Air config lost unrelated setting:\n%s", recovered)
	}
	if _, err := os.Stat(codexStatePath(path)); !os.IsNotExist(err) {
		t.Fatalf("stale Air sidecar remains after recovery: %v", err)
	}
}

func TestReconcileCodexConfigsTreatsAlreadyExternalAirRecoveryAsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("model_provider = \"jetbrains\"\nmodel = \"jb-default\"\nuser_setting = true\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	target := airStaleFullSelectionRecoveryTarget(path)
	plans, err := PlanCodexReconciliation(nil, []CodexTargetRef{target}, domain.Runtime{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Action != "already-external" {
		t.Fatalf("plans = %#v", plans)
	}
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{target}, domain.Runtime{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("external Air config changed: %q, %v", after, err)
	}
	if _, err := os.Stat(codexStatePath(path)); !os.IsNotExist(err) {
		t.Fatalf("already-external recovery created a sidecar: %v", err)
	}
}

func TestReconcileCodexConfigsRejectsUnsafeStaleAirFullSelectionRecovery(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, path string)
	}{
		{
			name: "full-selection-sidecar",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				block, err := codexManagedBlockIn(string(data))
				if err != nil {
					t.Fatal(err)
				}
				state, err := json.Marshal(codexState{
					ManagedBlockHash: hashText(block),
					ProjectionMode:   CodexProjectionFullSelection,
					WriterID:         CodexProjectionWriterID,
					TransactionID:    "full-selection-sidecar",
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(codexStatePath(path), append(state, '\n'), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "foreign-sidecar",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				stateData, err := os.ReadFile(codexStatePath(path))
				if err != nil {
					t.Fatal(err)
				}
				var state codexState
				if err := json.Unmarshal(stateData, &state); err != nil {
					t.Fatal(err)
				}
				state.WriterID = "foreign-projector"
				encoded, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(codexStatePath(path), append(encoded, '\n'), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "duplicate-full-selection-marker",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, []byte(codexBegin+"\n")...), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "quoted-duplicate-provider-selection",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				changed := strings.Replace(string(data), codexBegin, `"model_provider" = "aigw"`+"\n"+codexBegin, 1)
				if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "quoted-duplicate-model-selection",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				changed := strings.Replace(string(data), codexBegin, `"model" = "gpt-5.6-sol"`+"\n"+codexBegin, 1)
				if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "quoted-foreign-provider-table",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString("\n[model_providers.\"aigw\"]\nforeign = true\n"); err != nil {
					_ = file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "changed-provider-block",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				changed := strings.Replace(string(data), `wire_api = "responses"`, `wire_api = "foreign"`, 1)
				if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "fallback-sidecar-with-original-selection",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				stateData, err := os.ReadFile(codexStatePath(path))
				if err != nil {
					t.Fatal(err)
				}
				var state codexState
				if err := json.Unmarshal(stateData, &state); err != nil {
					t.Fatal(err)
				}
				state.OriginalProvider = `model_provider = "jetbrains"`
				encoded, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(codexStatePath(path), append(encoded, '\n'), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			runtime := atomicTestRuntime()
			fullBlock := codexManagedBlock(runtime.ProfileLabel, runtime.Endpoint)
			if err := os.WriteFile(path, []byte(projectCodex("model_provider = \"jetbrains\"\nmodel = \"jb-default\"\n", fullBlock, runtime.Model)), 0o600); err != nil {
				t.Fatal(err)
			}
			fallbackBlock := codexFallbackBlock(runtime.ProfileLabel, runtime.Endpoint)
			state, err := json.Marshal(codexState{
				ManagedBlockHash: hashText(fallbackBlock),
				ProjectionMode:   CodexProjectionNamespacedFallback,
				WriterID:         CodexProjectionWriterID,
				TransactionID:    "stale-air-full-selection",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(codexStatePath(path), append(state, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, path)
			configBefore, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			stateBefore, err := os.ReadFile(codexStatePath(path))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{airStaleFullSelectionRecoveryTarget(path)}, domain.Runtime{}); err == nil {
				t.Fatal("stale Air recovery unexpectedly accepted unsafe state")
			}
			configAfter, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(configAfter, configBefore) {
				t.Fatalf("config changed after rejected recovery: %q, %v", configAfter, err)
			}
			stateAfter, err := os.ReadFile(codexStatePath(path))
			if err != nil || !bytes.Equal(stateAfter, stateBefore) {
				t.Fatalf("sidecar changed after rejected recovery: %q, %v", stateAfter, err)
			}
		})
	}
}

func TestReconcileCodexConfigsRejectsNormalAirFallbackForStaleRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("model_provider = \"jetbrains\"\nmodel = \"jb-default\"\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{airFallbackCodexTarget(path)}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	configBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(codexStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{airStaleFullSelectionRecoveryTarget(path)}, domain.Runtime{}); err == nil {
		t.Fatal("stale Air recovery unexpectedly accepted a normal fallback")
	}
	configAfter, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(configAfter, configBefore) {
		t.Fatalf("config changed after rejected normal fallback: %q, %v", configAfter, err)
	}
	stateAfter, err := os.ReadFile(codexStatePath(path))
	if err != nil || !bytes.Equal(stateAfter, stateBefore) {
		t.Fatalf("sidecar changed after rejected normal fallback: %q, %v", stateAfter, err)
	}
}

func TestReconcileCodexConfigsRollsBackStaleAirRecoveryWhenSidecarRemovalFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	runtime := atomicTestRuntime()
	fullBlock := codexManagedBlock(runtime.ProfileLabel, runtime.Endpoint)
	if err := os.WriteFile(path, []byte(projectCodex("model_provider = \"jetbrains\"\nmodel = \"jb-default\"\nuser_setting = true\n", fullBlock, runtime.Model)), 0o600); err != nil {
		t.Fatal(err)
	}
	fallbackBlock := codexFallbackBlock(runtime.ProfileLabel, runtime.Endpoint)
	state, err := json.Marshal(codexState{
		ManagedBlockHash: hashText(fallbackBlock),
		ProjectionMode:   CodexProjectionNamespacedFallback,
		WriterID:         CodexProjectionWriterID,
		TransactionID:    "stale-air-full-selection",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(path), append(state, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	configBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(codexStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	originalRemove := removeFileIfUnchanged
	defer func() { removeFileIfUnchanged = originalRemove }()
	removeFileIfUnchanged = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		return transaction.FileSnapshot{}, errors.New("injected stale-Air sidecar removal failure")
	}
	_, err = ReconcileCodexConfigs(nil, []CodexTargetRef{airStaleFullSelectionRecoveryTarget(path)}, domain.Runtime{})
	if err == nil || !strings.Contains(err.Error(), "injected stale-Air sidecar removal failure") {
		t.Fatalf("ReconcileCodexConfigs() error = %v", err)
	}
	configAfter, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(configAfter, configBefore) {
		t.Fatalf("config after rollback = %q, %v", configAfter, err)
	}
	stateAfter, err := os.ReadFile(codexStatePath(path))
	if err != nil || !bytes.Equal(stateAfter, stateBefore) {
		t.Fatalf("sidecar after rollback = %q, %v", stateAfter, err)
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

func TestReconcileCodexConfigsUsesLegacySidecarBesideSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(realDir, "config.toml")
	aliasPath := filepath.Join(dir, "alias.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-native\"\n"
	if err := os.WriteFile(realPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, aliasPath); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime.ProfileLabel, runtime.Endpoint)
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

	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{standaloneCodexTarget(aliasPath)}, runtime); err != nil {
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
	if state.WriterID != CodexProjectionWriterID || state.ProjectionMode != CodexProjectionFullSelection || state.TransactionID == "" {
		t.Fatalf("legacy symlink sidecar was not adopted: %#v", state)
	}
}

func TestReconcileCodexConfigsRejectsUnownedAirFallbackBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("model_provider = \"jetbrains\"\n\n[model_providers.aigw_fallback]\nname = \"foreign\"\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReconcileCodexConfigs(nil, []CodexTargetRef{airFallbackCodexTarget(path)}, atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "unowned") {
		t.Fatalf("ReconcileCodexConfigs() error = %v, want unowned fallback conflict", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("config changed after unowned fallback rejection: %q, %v", got, readErr)
	}
}

func TestValidateCodexConfigRejectsForeignSidecarAttribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	if err := SyncCodexConfig(path, runtime); err != nil {
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
	err = ValidateCodexConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "foreign writer") {
		t.Fatalf("ValidateCodexConfig() error = %v, want foreign writer", err)
	}
}

func TestReadCodexProjectionIdentityDistinguishesFallbackAndLegacyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"jetbrains\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{airFallbackCodexTarget(path)}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	identity, err := ReadCodexProjectionIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if !identity.Present || identity.ProjectionMode != CodexProjectionNamespacedFallback || identity.AttributionState != "recognized" {
		t.Fatalf("fallback identity = %#v", identity)
	}
	stateData, err := os.ReadFile(codexStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}
	state.ProjectionMode, state.WriterID, state.TransactionID = "", "", ""
	legacyData, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(path), append(legacyData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err = ReadCodexProjectionIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if !identity.Present || identity.ProjectionMode != CodexProjectionFullSelection || identity.AttributionState != "legacy" {
		t.Fatalf("legacy identity = %#v", identity)
	}
}
