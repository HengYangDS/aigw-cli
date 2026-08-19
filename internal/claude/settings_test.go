package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

func testExecutable() string {
	return filepath.Join(os.TempDir(), "aigw-test-executable")
}

func TestSettingsReconcilePreservesForeignContentAndKeepsCredentialsOutOfJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := []byte(`{
  "permissions": {"allow": ["Read"]},
  "env": {"TEAM_VALUE": "kept"},
  "model": "foreign-model"
}
`)
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := configuration.Runtime{
		ProfileID: "team-claude", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team",
	}

	executable := filepath.Join(t.TempDir(), "AIGW CLI", "aigw")
	receipt, err := ReconcileSettings(path, false, runtime, executable)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Action != "project" {
		t.Fatalf("action = %q", receipt.Action)
	}
	var got map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatalf("settings contain credential material: %s", data)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["permissions"], map[string]any{"allow": []any{"Read"}}) {
		t.Fatalf("foreign permissions changed: %#v", got["permissions"])
	}
	env := got["env"].(map[string]any)
	if env["TEAM_VALUE"] != "kept" || env["ANTHROPIC_BASE_URL"] != "https://gateway.test" {
		t.Fatalf("env = %#v", env)
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Fatalf("stale credential remained: %#v", env)
	}
	if got["model"] != "claude-team" || got["apiKeyHelper"] != credentialHelper(executable) {
		t.Fatalf("managed settings = %#v", got)
	}

	state, err := loadSettingsState(path + settingsStateSuffix)
	if err != nil {
		t.Fatal(err)
	}
	if string(state.Original.Model.Value) != `"foreign-model"` {
		t.Fatalf("original model was not captured: %#v", state.Original)
	}
}

func TestSettingsRejectsRelativeExecutableAndProjectsAbsoluteHelper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	if _, err := ReconcileSettings(path, false, runtime, "aigw"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative executable error = %v", err)
	}
	executable := filepath.Join(t.TempDir(), "AIGW CLI", "aigw")
	if _, err := ReconcileSettings(path, false, runtime, executable); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if got, want := document["apiKeyHelper"], credentialHelper(executable); got != want {
		t.Fatalf("apiKeyHelper = %#v, want %#v", got, want)
	}
}

func TestSettingsRejectsControlCharactersInExecutablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	absolute := filepath.Join(t.TempDir(), "aigw")
	for _, executable := range []string{absolute + "\x00", absolute + "\n"} {
		if _, err := ReconcileSettings(path, false, runtime, executable); err == nil || !strings.Contains(err.Error(), "control") {
			t.Fatalf("executable %q error = %v", executable, err)
		}
	}
}

func TestSettingsDisableRestoresOnlyCapturedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := []byte(`{"theme":"dark","env":{"TEAM_VALUE":"kept"},"model":"native"}` + "\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileSettings(path, true, configuration.Runtime{}, ""); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wantJSON, gotJSON any
	if err := json.Unmarshal(before, &wantJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("restored settings = %s, want semantic %s", got, before)
	}
	if _, err := os.Stat(path + settingsStateSuffix); !os.IsNotExist(err) {
		t.Fatalf("state remains after disable: %v", err)
	}
}

func TestSettingsDisableRestoresAnAbsentSettingsFileToAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileSettings(path, true, configuration.Runtime{}, ""); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{path, path + settingsStateSuffix} {
		if _, err := os.Stat(candidate); !os.IsNotExist(err) {
			t.Fatalf("%s remains: %v", candidate, err)
		}
	}
}

func TestSettingsUpdatePreservesForeignEditsMadeAfterProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	document["theme"] = "dark"
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Endpoint = "https://next.test"
	if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document["theme"] != "dark" {
		t.Fatalf("foreign edit was not preserved: %#v", document)
	}
	environment := document["env"].(map[string]any)
	if environment["ANTHROPIC_BASE_URL"] != "https://next.test" {
		t.Fatalf("managed endpoint was not updated: %#v", environment)
	}
}

func TestSettingsRejectsForeignMutationOfManagedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "https://gateway.test", "https://foreign.test", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ReconcileSettings(path, false, runtime, testExecutable())
	if err == nil || !strings.Contains(err.Error(), "managed Claude settings changed") {
		t.Fatalf("error = %v", err)
	}
}

func TestSettingsRejectsPlaintextCredentialOrForeignHelperWithoutWriting(t *testing.T) {
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	for _, content := range []string{
		`{"env":{"ANTHROPIC_AUTH_TOKEN":"plaintext"}}`,
		`{"env":{"ANTHROPIC_API_KEY":"plaintext"}}`,
		`{"apiKeyHelper":"foreign-helper"}`,
	} {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ReconcileSettings(path, false, runtime, testExecutable())
		if err == nil || !strings.Contains(err.Error(), "credential ownership conflict") {
			t.Fatalf("error = %v", err)
		}
		if _, err := os.Stat(path + settingsStateSuffix); !os.IsNotExist(err) {
			t.Fatalf("state written after rejection: %v", err)
		}
	}
}

func TestSettingsProjectionIsIdempotentAndRejectsInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	first, err := ReconcileSettings(path, false, runtime, testExecutable())
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReconcileSettings(path, false, runtime, testExecutable())
	if err != nil {
		t.Fatal(err)
	}
	if first.Action != "project" || second.Action != "already-converged" {
		t.Fatalf("actions = %q, %q", first.Action, second.Action)
	}
	for _, test := range []struct {
		name    string
		path    string
		runtime configuration.Runtime
		want    string
	}{
		{name: "missing path", runtime: runtime, want: "settings path"},
		{name: "missing endpoint", path: path, runtime: configuration.Runtime{ProfileID: "team", AccountID: "gateway"}, want: "no Claude endpoint"},
		{name: "missing account", path: path, runtime: configuration.Runtime{ProfileID: "team", Endpoint: "https://gateway.test"}, want: "no account"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReconcileSettings(test.path, false, test.runtime, testExecutable())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestSettingsDisableWithoutOwnedStateIsAlreadyRestored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	receipt, err := ReconcileSettings(path, true, configuration.Runtime{}, "")
	if err != nil || receipt.Action != "already-restored" {
		t.Fatalf("receipt=%#v error=%v", receipt, err)
	}
}

func TestSettingsNullDocumentBecomesAnEmptyObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsStrictlyRejectsMalformedEnvironmentAndTrailingJSON(t *testing.T) {
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	for name, content := range map[string]string{
		"malformed document": `{`,
		"malformed env":      `{"env":"not-an-object"}`,
		"trailing value":     `{"theme":"dark"} {"theme":"light"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			before := []byte(content)
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err == nil {
				t.Fatal("invalid settings accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Fatalf("settings changed: %q error=%v", after, err)
			}
		})
	}
}

func TestSettingsRejectsMalformedOwnedStateForUpdateAndDisable(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "update", true: "disable"}[disabled], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+settingsStateSuffix, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, disabled, runtime, testExecutable()); err == nil || !strings.Contains(err.Error(), "parse Claude settings state") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestSettingsTransactionFailuresRollbackOrReportExactCause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}

	t.Run("settings snapshot", func(t *testing.T) {
		withSettingsTransaction(t,
			func(string) (transaction.FileSnapshot, error) { return transaction.FileSnapshot{}, os.ErrPermission },
			writeGuarded, removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err == nil || !strings.Contains(err.Error(), "read Claude settings") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state snapshot", func(t *testing.T) {
		calls := 0
		withSettingsTransaction(t,
			func(path string) (transaction.FileSnapshot, error) {
				calls++
				if calls == 2 {
					return transaction.FileSnapshot{}, os.ErrPermission
				}
				return transaction.CaptureFileSnapshot(path)
			},
			writeGuarded, removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err == nil || !strings.Contains(err.Error(), "read Claude settings state") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("settings write", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot,
			func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error) {
				return transaction.FileSnapshot{}, os.ErrPermission
			},
			removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err == nil || !strings.Contains(err.Error(), "write Claude settings") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state write rollback", func(t *testing.T) {
		calls := 0
		rolledBack := false
		withSettingsTransaction(t, captureSnapshot,
			func(path string, before transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
				calls++
				if calls == 2 {
					return transaction.FileSnapshot{}, os.ErrPermission
				}
				return transaction.WriteFileAtomicIfUnchanged(path, before, data, mode)
			},
			removeGuarded,
			func(path string, before, after transaction.FileSnapshot) error {
				rolledBack = true
				return transaction.RestoreFileAtomicIfPostimage(path, before, after)
			},
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err == nil || !strings.Contains(err.Error(), "write Claude settings state") || !rolledBack {
			t.Fatalf("error=%v rolledBack=%t", err, rolledBack)
		}
	})

	t.Run("state write rollback failure", func(t *testing.T) {
		calls := 0
		withSettingsTransaction(t, captureSnapshot,
			func(path string, before transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
				calls++
				if calls == 2 {
					return transaction.FileSnapshot{}, os.ErrPermission
				}
				return transaction.WriteFileAtomicIfUnchanged(path, before, data, mode)
			},
			removeGuarded,
			func(string, transaction.FileSnapshot, transaction.FileSnapshot) error {
				return errors.New("rollback failed")
			},
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err == nil || !strings.Contains(err.Error(), "rollback failed") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestSettingsDisableFailuresPreserveManagedProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
		t.Fatal(err)
	}

	t.Run("managed drift", func(t *testing.T) {
		data, _ := os.ReadFile(path)
		if err := os.WriteFile(path, bytes.Replace(data, []byte("gateway.test"), []byte("foreign.test"), 1), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileSettings(path, true, configuration.Runtime{}, ""); err == nil || !strings.Contains(err.Error(), "refusing to remove") {
			t.Fatalf("error=%v", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("restore write", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot,
			func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error) {
				return transaction.FileSnapshot{}, os.ErrPermission
			},
			removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, true, configuration.Runtime{}, ""); err == nil || !strings.Contains(err.Error(), "restore Claude settings") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state removal and rollback", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot, writeGuarded,
			func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
				return transaction.FileSnapshot{}, os.ErrPermission
			},
			restoreGuarded,
		)
		if _, err := ReconcileSettings(path, true, configuration.Runtime{}, ""); err == nil || !strings.Contains(err.Error(), "remove Claude settings state") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state removal rollback failure", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot, writeGuarded,
			func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
				return transaction.FileSnapshot{}, os.ErrPermission
			},
			func(string, transaction.FileSnapshot, transaction.FileSnapshot) error {
				return errors.New("rollback failed")
			},
		)
		if _, err := ReconcileSettings(path, true, configuration.Runtime{}, ""); err == nil || !strings.Contains(err.Error(), "rollback failed") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestSettingsDisableAbsentFileReportsRemovalFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprintf("remove-%d", failAt), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable()); err != nil {
				t.Fatal(err)
			}
			calls := 0
			withSettingsTransaction(t, captureSnapshot, writeGuarded,
				func(path string, before transaction.FileSnapshot) (transaction.FileSnapshot, error) {
					calls++
					if calls == failAt {
						return transaction.FileSnapshot{}, os.ErrPermission
					}
					return transaction.RemoveFileIfUnchanged(path, before)
				},
				restoreGuarded,
			)
			if _, err := ReconcileSettings(path, true, configuration.Runtime{}, ""); err == nil {
				t.Fatal("removal failure was accepted")
			}
		})
	}
}

func TestSettingsStateValidationAndHelperBranches(t *testing.T) {
	valid := settingsState{Version: 1, WriterID: "aigw-cli", ManagedSHA256: "digest", Original: originalSettings{Environment: map[string]optionalValue{}}}
	data := encodeSettingsState(valid)
	for name, stateData := range map[string][]byte{
		"malformed":  []byte("{"),
		"unknown":    []byte(`{"version":1,"writer_id":"aigw-cli","managed_sha256":"digest","extra":true}`),
		"incomplete": []byte(`{"version":1,"writer_id":"other"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeSettingsState(stateData); err == nil {
				t.Fatal("invalid state accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSettingsState(path); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSettingsState(path + ".missing"); err == nil {
		t.Fatal("missing state accepted")
	}

	document := settingsDocument{"env": json.RawMessage("null")}
	environment, err := decodeEnvironment(document)
	if err != nil || len(environment) != 0 {
		t.Fatalf("environment=%v error=%v", environment, err)
	}
	if !hasCredentialOwnershipConflict(settingsDocument{"env": json.RawMessage(`"bad"`)}) {
		t.Fatal("malformed environment did not fail closed")
	}
	projectSettings(document, configuration.Runtime{Endpoint: "https://gateway.test"}, testExecutable())
	if _, ok := document["model"]; ok {
		t.Fatal("empty model was retained")
	}
	projectSettings(settingsDocument{}, configuration.Runtime{Endpoint: "https://gateway.test"}, testExecutable())
	original := captureOriginalSettings(settingsDocument{"env": json.RawMessage(`{"ANTHROPIC_MODEL":"legacy"}`)}, true)
	if !original.Environment["ANTHROPIC_MODEL"].Present {
		t.Fatal("managed environment value was not captured")
	}
	restored := settingsDocument{}
	restoreOriginalSettings(restored, original)
	if _, ok := restored["env"]; !ok {
		t.Fatal("managed environment value was not restored")
	}
}

func withSettingsTransaction(
	t *testing.T,
	capture func(string) (transaction.FileSnapshot, error),
	write func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error),
	remove func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error),
	restore func(string, transaction.FileSnapshot, transaction.FileSnapshot) error,
) {
	t.Helper()
	oldCapture, oldWrite, oldRemove, oldRestore := captureSnapshot, writeGuarded, removeGuarded, restoreGuarded
	captureSnapshot, writeGuarded, removeGuarded, restoreGuarded = capture, write, remove, restore
	t.Cleanup(func() {
		captureSnapshot, writeGuarded, removeGuarded, restoreGuarded = oldCapture, oldWrite, oldRemove, oldRestore
	})
}
