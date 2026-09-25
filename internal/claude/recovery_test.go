package claude

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSettingsReconcilesOnlyProvenModelPreferenceDrift(t *testing.T) {
	for _, changedModel := range []string{"", `"user-selected-model"`} {
		t.Run(changedModel, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			selected := configuration.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
			if _, err := ReconcileSettings(path, false, selected, testExecutable(), selected.Model); err != nil {
				t.Fatal(err)
			}
			document := readSettingsFile(t, path)
			delete(document, "model")
			if changedModel != "" {
				document["model"] = json.RawMessage(changedModel)
			}
			document["theme"] = json.RawMessage(`"dark"`)
			before := encodeSettings(document)
			before = bytes.ReplaceAll(before, []byte("/"), []byte(`\/`))
			before = bytes.ReplaceAll(before, []byte("user-selected-model"), []byte(`\u0075ser-selected-model`))
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			plan, err := PlanSettings(path, false, selected, testExecutable(), selected.Model)
			if err != nil || plan.Action != SettingsActionAlreadyConverged {
				t.Fatalf("recoverable model drift: %+v, %v", plan, err)
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, before) {
				t.Fatal("preview changed settings")
			}
			receipt, err := ReconcileSettings(path, false, selected, testExecutable(), selected.Model)
			if err != nil || receipt.Action != SettingsActionAlreadyConverged {
				t.Fatalf("unchanged connection reconciliation = %#v, %v", receipt, err)
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, before) {
				t.Fatal("unchanged connection rewrote native model preference")
			}
			if err := receipt.Rollback(); err != nil {
				t.Fatal(err)
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, before) {
				t.Fatal("rollback lost observed user preference")
			}
		})
	}
}

func TestHelperOnlyMigrationRetainsNativeModelPreference(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	oldHelper := filepath.Join(root, "old-aigw")
	newHelper := filepath.Join(root, "new-aigw")
	runtime := configuration.Runtime{
		RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-opus-5-5",
	}
	if _, err := ReconcileSettings(path, false, runtime, oldHelper, ""); err != nil {
		t.Fatal(err)
	}
	document := readSettingsFile(t, path)
	document["model"] = encodeRaw("opus[1m]")
	before := encodeSettings(document)
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(path + settingsStateSuffix)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanSettings(path, false, runtime, newHelper, runtime.Model)
	if err != nil || plan.Action != SettingsActionProject {
		t.Fatalf("helper-only plan = %#v, %v", plan, err)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, before) {
		t.Fatal("helper-only preview changed settings")
	}
	receipt, err := ReconcileSettings(path, false, runtime, newHelper, runtime.Model)
	if err != nil {
		t.Fatal(err)
	}
	projected := readSettingsFile(t, path)
	if string(projected["model"]) != string(encodeRaw("opus[1m]")) {
		t.Fatalf("native model choice became %s", projected["model"])
	}
	if string(projected["apiKeyHelper"]) != string(encodeRaw(credentialHelper(newHelper, runtime.CredentialProjectionFingerprint(configuration.ClientClaude)))) {
		t.Fatal("helper migration did not project the new executable")
	}
	if inspection, err := InspectSettings(path, runtime, newHelper); err != nil || !inspection.NativeModelOverride {
		t.Fatalf("native model preference inspection = %#v, %v", inspection, err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, before) {
		t.Fatal("helper rollback lost user settings")
	}
	if data, err := os.ReadFile(path + settingsStateSuffix); err != nil || !bytes.Equal(data, stateBefore) {
		t.Fatal("helper rollback lost ownership state")
	}
}

func TestHelperMigrationDoesNotCarryModelPreferenceAcrossRouteChanges(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*configuration.Runtime)
	}{
		{name: "account", change: func(runtime *configuration.Runtime) { runtime.AccountID = "another" }},
		{name: "endpoint", change: func(runtime *configuration.Runtime) { runtime.Endpoint = "https://another.test" }},
		{name: "model", change: func(runtime *configuration.Runtime) { runtime.Model = "claude-sonnet-5" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "settings.json")
			previous := configuration.Runtime{
				RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-opus-5-5",
			}
			if _, err := ReconcileSettings(path, false, previous, filepath.Join(root, "old-aigw"), ""); err != nil {
				t.Fatal(err)
			}
			document := readSettingsFile(t, path)
			document["model"] = encodeRaw("opus[1m]")
			if err := os.WriteFile(path, encodeSettings(document), 0o600); err != nil {
				t.Fatal(err)
			}
			next := previous
			test.change(&next)
			newHelper := filepath.Join(root, "new-aigw")
			if _, err := ReconcileSettings(path, false, next, newHelper, previous.Model); err != nil {
				t.Fatal(err)
			}
			if got := readSettingsFile(t, path)["model"]; string(got) != string(encodeRaw(next.Model)) {
				t.Fatalf("route change retained a native preference: %s", got)
			}
			if inspection, err := InspectSettings(path, next, newHelper); err != nil || inspection.NativeModelOverride {
				t.Fatalf("route change inspection = %#v, %v", inspection, err)
			}
		})
	}
}

func TestSettingsWithdrawalPreservesLaterModelPreference(t *testing.T) {
	for _, test := range []struct {
		name      string
		reproject bool
		model     string
	}{
		{name: "remove model"},
		{name: "preserve model", model: `"user-model"`},
		{name: "remove model after reprojection", reproject: true},
		{name: "preserve model after reprojection", reproject: true, model: `"user-model"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(`{"model":"original","theme":"dark"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			runtime := configuration.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
				t.Fatal(err)
			}
			document := readSettingsFile(t, path)
			want := settingsDocument{"theme": json.RawMessage(`"dark"`)}
			delete(document, "model")
			if test.model != "" {
				document["model"] = json.RawMessage(test.model)
				want["model"] = json.RawMessage(test.model)
			}
			if err := os.WriteFile(path, encodeSettings(document), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.reproject {
				if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := ReconcileSettings(path, true, configuration.Runtime{}, "", runtime.Model); err != nil {
				t.Fatal(err)
			}
			if got := readSettingsFile(t, path); !reflect.DeepEqual(got, want) {
				t.Fatalf("withdrawal lost user preference or retained owned fields: %s", encodeSettings(got))
			}
			if _, err := os.Stat(path + settingsStateSuffix); !os.IsNotExist(err) {
				t.Fatalf("ownership state remains: %v", err)
			}
		})
	}
}

func TestSettingsModelRecoveryRequiresUnchangedConnectionOwnership(t *testing.T) {
	for _, field := range []string{"apiKeyHelper", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL"} {
		t.Run(field, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			runtime := configuration.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test", Model: "claude-team"}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
				t.Fatal(err)
			}
			document := readSettingsFile(t, path)
			delete(document, "model")
			if field == "apiKeyHelper" {
				document[field] = encodeRaw("foreign")
			} else {
				env, err := decodeEnvironment(document)
				if err != nil {
					t.Fatal(err)
				}
				env[field] = encodeRaw("foreign")
				document["env"], err = json.Marshal(env)
				if err != nil {
					t.Fatal(err)
				}
			}
			before := encodeSettings(document)
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			stateBefore, err := os.ReadFile(path + settingsStateSuffix)
			if err != nil {
				t.Fatal(err)
			}
			for _, disabled := range []bool{false, true} {
				if _, err := ReconcileSettings(path, disabled, runtime, testExecutable(), runtime.Model); err == nil {
					t.Fatal("connection conflict accepted as model preference")
				}
			}
			if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, before) {
				t.Fatal("conflict changed settings")
			}
			if data, err := os.ReadFile(path + settingsStateSuffix); err != nil || !bytes.Equal(data, stateBefore) {
				t.Fatal("conflict adopted foreign ownership")
			}
		})
	}
}

func TestSettingsReceiptRestoresExactObservedFiles(t *testing.T) {
	for _, test := range []struct {
		action                        string
		existing, projected, disabled bool
	}{
		{action: "project", existing: true},
		{action: "restore-existing", existing: true, projected: true, disabled: true},
		{action: "restore-absent", projected: true, disabled: true},
		{action: "already-converged", existing: true, projected: true},
		{action: "already-restored", disabled: true},
	} {
		t.Run(test.action, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if test.existing {
				if err := os.WriteFile(path, []byte(`{ "theme":"dark", "env":{"TEAM_VALUE":"kept"}}`), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			runtime := configuration.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
			if test.projected {
				if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
					t.Fatal(err)
				}
			}
			paths := []string{path, path + settingsStateSuffix}
			before := make(map[string]transaction.FileSnapshot)
			for _, target := range paths {
				snapshot, err := transaction.CaptureFileSnapshot(target)
				if err != nil {
					t.Fatal(err)
				}
				before[target] = snapshot
			}
			receipt, err := ReconcileSettings(path, test.disabled, runtime, testExecutable(), runtime.Model)
			if err != nil {
				t.Fatal(err)
			}
			if err := receipt.Rollback(); err != nil {
				t.Fatal(err)
			}
			for _, target := range paths {
				after, err := transaction.CaptureFileSnapshot(target)
				if err != nil || !reflect.DeepEqual(after, before[target]) {
					t.Fatalf("%s rollback snapshot = %#v, %v; want %#v", target, after, err, before[target])
				}
			}
			encoded, err := json.Marshal(receipt)
			if err != nil || bytes.Contains(encoded, []byte("TEAM_VALUE")) || bytes.Contains(encoded, []byte("gateway.test")) {
				t.Fatalf("receipt must expose metadata only: %s, %v", encoded, err)
			}
		})
	}
}

func TestSettingsReceiptPreservesNewerFiles(t *testing.T) {
	for _, suffix := range []string{"", settingsStateSuffix} {
		t.Run("edited"+suffix, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			runtime := configuration.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
			receipt, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model)
			if err != nil {
				t.Fatal(err)
			}
			foreign := []byte(`{"owner":"newer-writer"}`)
			if err := os.WriteFile(path+suffix, foreign, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := receipt.Rollback(); err == nil || !strings.Contains(err.Error(), "postimage changed") {
				t.Fatalf("Rollback() error = %v", err)
			}
			actual, err := os.ReadFile(path + suffix)
			if err != nil || !bytes.Equal(actual, foreign) {
				t.Fatalf("newer file = %s, %v", actual, err)
			}
			owned := path
			if suffix == "" {
				owned += settingsStateSuffix
			}
			if _, err := os.Stat(owned); !os.IsNotExist(err) {
				t.Fatalf("independent owned file was not restored to absence: %s, %v", owned, err)
			}
		})
	}
}
