package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestInspectSettingsPreservesSidecarProvenNativeModelPreference(t *testing.T) {
	tests := []struct {
		name string
		edit func(settingsDocument)
	}{
		{
			name: "different native model",
			edit: func(document settingsDocument) {
				document["model"] = encodeRaw("opus[1m]")
			},
		},
		{
			name: "native model removed",
			edit: func(document settingsDocument) {
				delete(document, "model")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			selected := configuration.Runtime{RouteID: "fable", AccountID: "ucloud", Endpoint: "https://ucloud.test", Model: "claude-fable-5-1"}
			if _, err := ReconcileSettings(path, false, selected, testExecutable(), selected.Model); err != nil {
				t.Fatal(err)
			}
			document := readSettingsFile(t, path)
			test.edit(document)
			if err := os.WriteFile(path, encodeSettings(document), 0o600); err != nil {
				t.Fatal(err)
			}
			beforeSettings, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			beforeSidecar, err := os.ReadFile(path + settingsStateSuffix)
			if err != nil {
				t.Fatal(err)
			}

			inspection, err := InspectSettings(path, selected, testExecutable())
			if err != nil || !inspection.NativeModelOverride {
				t.Fatalf("inspection = %#v, %v", inspection, err)
			}
			if err := ValidateSettings(path, selected, testExecutable()); err != nil {
				t.Fatalf("model-only native preference became invalid: %v", err)
			}
			plan, err := PlanSettings(path, false, selected, testExecutable(), selected.Model)
			if err != nil || plan.Action != SettingsActionProject {
				t.Fatalf("explicit sync preview = %#v, %v", plan, err)
			}
			afterSettings, _ := os.ReadFile(path)
			afterSidecar, _ := os.ReadFile(path + settingsStateSuffix)
			if !bytes.Equal(beforeSettings, afterSettings) || !bytes.Equal(beforeSidecar, afterSidecar) {
				t.Fatal("inspection or preview rewrote Claude settings or ownership state")
			}
		})
	}
}

func TestInspectSettingsRejectsConnectionEditsAlongsideModelPreference(t *testing.T) {
	tests := []struct {
		name string
		edit func(settingsDocument)
	}{
		{
			name: "foreign helper",
			edit: func(document settingsDocument) {
				document["apiKeyHelper"] = encodeRaw("foreign-helper")
			},
		},
		{
			name: "foreign endpoint",
			edit: func(document settingsDocument) {
				var environment map[string]json.RawMessage
				if err := json.Unmarshal(document["env"], &environment); err != nil {
					panic(err)
				}
				environment["ANTHROPIC_BASE_URL"] = encodeRaw("https://foreign.test")
				document["env"], _ = json.Marshal(environment)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			selected := configuration.Runtime{RouteID: "fable", AccountID: "ucloud", Endpoint: "https://ucloud.test", Model: "claude-fable-5-1"}
			if _, err := ReconcileSettings(path, false, selected, testExecutable(), selected.Model); err != nil {
				t.Fatal(err)
			}
			document := readSettingsFile(t, path)
			document["model"] = encodeRaw("opus[1m]")
			test.edit(document)
			if err := os.WriteFile(path, encodeSettings(document), 0o600); err != nil {
				t.Fatal(err)
			}
			beforeSettings, _ := os.ReadFile(path)
			beforeSidecar, _ := os.ReadFile(path + settingsStateSuffix)

			_, err := InspectSettings(path, selected, testExecutable())
			if err == nil || !strings.Contains(err.Error(), "not synchronized") {
				t.Fatalf("connection conflict error = %v", err)
			}
			afterSettings, _ := os.ReadFile(path)
			afterSidecar, _ := os.ReadFile(path + settingsStateSuffix)
			if !bytes.Equal(beforeSettings, afterSettings) || !bytes.Equal(beforeSidecar, afterSidecar) {
				t.Fatal("inspection rewrote a conflicting client file")
			}
		})
	}
}
