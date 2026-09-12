package claude

import (
	configuration "aigw-cli/internal/configuration"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testExecutable() string {
	return filepath.Join(os.TempDir(), "aigw-test-executable")
}

func readSettingsFile(t *testing.T, path string) settingsDocument {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document settingsDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestSettingsOwnershipUsesStringValuesNotJSONEscapes(t *testing.T) {
	for _, removedModel := range []bool{false, true} {
		t.Run(fmt.Sprintf("removed-model=%t", removedModel), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			escaped := bytes.ReplaceAll(before, []byte("/"), []byte(`\/`))
			escaped = bytes.ReplaceAll(escaped, []byte("claude-team"), []byte(`\u0063laude-team`))
			if removedModel {
				var document settingsDocument
				if err := json.Unmarshal(escaped, &document); err != nil {
					t.Fatal(err)
				}
				delete(document, "model")
				escaped = encodeSettings(document)
			}
			if err := os.WriteFile(path, escaped, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := PlanSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
				t.Fatalf("equivalent JSON spelling became an ownership conflict: %v", err)
			}
			if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, escaped) {
				t.Fatal("preview rewrote the client's JSON")
			}
			receipt, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateSettings(path, runtime, testExecutable()); err != nil {
				t.Fatal(err)
			}
			if err := receipt.Rollback(); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, escaped) {
				t.Fatal("compensation lost the observed client's JSON spelling")
			}
		})
	}
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
	receipt, err := ReconcileSettings(path, false, runtime, executable, runtime.Model)
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
	if !reflect.DeepEqual(got["env"], map[string]any{
		"TEAM_VALUE":         "kept",
		"ANTHROPIC_BASE_URL": "https://gateway.test",
	}) {
		t.Fatalf("env = %#v", got["env"])
	}
	if got["model"] != "claude-team" || got["apiKeyHelper"] != credentialHelper(executable, runtime.CredentialProjectionFingerprint(configuration.ClientClaude)) {
		t.Fatalf("managed settings = %#v", got)
	}

	stateData, err := os.ReadFile(path + settingsStateSuffix)
	if err != nil {
		t.Fatal(err)
	}
	state, err := decodeSettingsState(stateData)
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
	if _, err := ReconcileSettings(path, false, runtime, "aigw", runtime.Model); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative executable error = %v", err)
	}
	executable := filepath.Join(t.TempDir(), "AIGW CLI", "aigw")
	if _, err := ReconcileSettings(path, false, runtime, executable, runtime.Model); err != nil {
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
	if got, want := document["apiKeyHelper"], credentialHelper(executable, runtime.CredentialProjectionFingerprint(configuration.ClientClaude)); got != want {
		t.Fatalf("apiKeyHelper = %#v, want %#v", got, want)
	}
}

func TestSettingsRejectsControlCharactersInExecutablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	absolute := filepath.Join(t.TempDir(), "aigw")
	for _, executable := range []string{absolute + "\x00", absolute + "\n"} {
		if _, err := ReconcileSettings(path, false, runtime, executable, runtime.Model); err == nil || !strings.Contains(err.Error(), "control") {
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
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileSettings(path, true, configuration.Runtime{}, "", ""); err != nil {
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
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileSettings(path, true, configuration.Runtime{}, "", ""); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{path, path + settingsStateSuffix} {
		if _, err := os.Stat(candidate); !os.IsNotExist(err) {
			t.Fatalf("%s remains: %v", candidate, err)
		}
	}
}

func TestSettingsLifecyclePreservesForeignEditsMadeAfterProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
		t.Fatal(err)
	}
	document := readSettingsFile(t, path)
	document["theme"] = json.RawMessage(`"dark"`)
	var environment map[string]json.RawMessage
	if err := json.Unmarshal(document["env"], &environment); err != nil {
		t.Fatal(err)
	}
	environment["TEAM_VALUE"] = json.RawMessage(`"kept"`)
	encodedEnvironment, err := json.Marshal(environment)
	if err != nil {
		t.Fatal(err)
	}
	document["env"] = encodedEnvironment
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Endpoint = "https://next.test"
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
		t.Fatal(err)
	}
	document = readSettingsFile(t, path)
	if string(document["theme"]) != `"dark"` {
		t.Fatalf("foreign edit was not preserved: %#v", document)
	}
	if err := json.Unmarshal(document["env"], &environment); err != nil {
		t.Fatal(err)
	}
	if string(environment["ANTHROPIC_BASE_URL"]) != `"https://next.test"` {
		t.Fatalf("managed endpoint was not updated: %#v", environment)
	}
	for range 2 {
		if _, err := ReconcileSettings(path, true, configuration.Runtime{}, "", ""); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("disable removed user-authored settings: %v", err)
		}
		var restored struct {
			Theme string            `json:"theme"`
			Env   map[string]string `json:"env"`
		}
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if restored.Theme != "dark" || !reflect.DeepEqual(restored.Env, map[string]string{"TEAM_VALUE": "kept"}) {
			t.Fatalf("disable did not retain exact user fields: %s", data)
		}
		if _, err := os.Stat(path + settingsStateSuffix); !os.IsNotExist(err) {
			t.Fatalf("disable retained owned state: %v", err)
		}
	}
}

func TestSettingsRejectsForeignMutationOfManagedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
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
	_, err = ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model)
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
		_, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model)
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
	first, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model)
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
			_, err := ReconcileSettings(test.path, false, test.runtime, testExecutable(), test.runtime.Model)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPlanSettingsMatchesApplyWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := configuration.Runtime{ProfileID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
	executable := testExecutable()

	plan, err := PlanSettings(path, false, runtime, executable, runtime.Model)
	if err != nil || plan.Action != "project" || plan.Target != path {
		t.Fatalf("initial plan = %#v, %v", plan, err)
	}
	for _, candidate := range []string{path, path + settingsStateSuffix} {
		if _, err := os.Stat(candidate); !os.IsNotExist(err) {
			t.Fatalf("planning wrote %s: %v", candidate, err)
		}
	}
	if _, err := ReconcileSettings(path, false, runtime, executable, runtime.Model); err != nil {
		t.Fatal(err)
	}
	settingsBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(path + settingsStateSuffix)
	if err != nil {
		t.Fatal(err)
	}

	plan, err = PlanSettings(path, false, runtime, executable, runtime.Model)
	if err != nil || plan.Action != "already-converged" {
		t.Fatalf("converged plan = %#v, %v", plan, err)
	}
	plan, err = PlanSettings(path, true, configuration.Runtime{}, "", "")
	if err != nil || plan.Action != "restore" {
		t.Fatalf("restore plan = %#v, %v", plan, err)
	}
	settingsAfter, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(settingsAfter, settingsBefore) {
		t.Fatalf("settings changed during planning: %q, %v", settingsAfter, err)
	}
	stateAfter, err := os.ReadFile(path + settingsStateSuffix)
	if err != nil || !bytes.Equal(stateAfter, stateBefore) {
		t.Fatalf("state changed during planning: %q, %v", stateAfter, err)
	}
}

func TestSettingsDisableWithoutOwnedStateIsAlreadyRestored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	receipt, err := ReconcileSettings(path, true, configuration.Runtime{}, "", "")
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
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
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
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err == nil {
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
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+settingsStateSuffix, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, disabled, runtime, testExecutable(), runtime.Model); err == nil || !strings.Contains(err.Error(), "parse Claude settings state") {
				t.Fatalf("error=%v", err)
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
	if _, err := decodeSettingsState(data); err != nil {
		t.Fatal(err)
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
