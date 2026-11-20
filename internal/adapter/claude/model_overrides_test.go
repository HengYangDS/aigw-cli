package claude

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// overridesRuntime is a runtime of Suhe.AI's Claude Code channel of Opus 5.5
// whose provider serves Fable 5.2 as its own channel too.
func overridesRuntime() domain.Runtime {
	return domain.Runtime{
		Model: domain.NewModelRef("suhe", "claude-code-opus-5-5"), ProviderID: "suhe", Endpoint: "https://suhe.test",
		UpstreamModel:   "claude-code-opus-5-5",
		AnthropicModels: map[string]string{"claude-opus-5-5": "claude-code-opus-5-5", "claude-fable-5-2": "claude-code-fable-5-2"},
	}
}

// writtenOverrides reads the map that the settings file holds, and whether it
// holds one.
func writtenOverrides(t *testing.T, path string) (map[string]string, bool) {
	t.Helper()
	raw, present := readSettingsFile(t, path)["modelOverrides"]
	if !present {
		return nil, false
	}
	var overrides map[string]string
	if err := json.Unmarshal(raw, &overrides); err != nil {
		t.Fatalf("modelOverrides is not a map of strings: %s", raw)
	}
	return overrides, true
}

// Claude Code's settings carry the runtime's map, so that every picker row
// and alias reaches the provider's ID: the projection writes it, writes it
// again unchanged, and leaves it out when the runtime has none.
func TestSettingsProjectTheProvidersModelOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := overridesRuntime()
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	if got, present := writtenOverrides(t, path); !present || !maps.Equal(got, runtime.AnthropicModels) {
		t.Fatalf("modelOverrides = %v (present %t), want %v", got, present, runtime.AnthropicModels)
	}
	plan, err := PlanSettings(path, false, runtime, testExecutable(), runtime.UpstreamModel)
	if err != nil || plan.Action != SettingsActionAlreadyConverged {
		t.Fatalf("the projected map plans %q, %v; want already converged", plan.Action, err)
	}
	if _, err := InspectSettings(path, runtime, testExecutable()); err != nil {
		t.Fatalf("inspection of the projected map: %v", err)
	}

	without := runtime
	without.AnthropicModels = nil
	if _, err := ReconcileSettings(path, false, without, testExecutable(), runtime.UpstreamModel); err != nil {
		t.Fatal(err)
	}
	if got, present := writtenOverrides(t, path); present {
		t.Fatalf("a runtime without a map left modelOverrides %v", got)
	}
}

// A map that the user wrote before AIGW comes back when AIGW withdraws, and
// settings that held none hold none afterwards.
func TestSettingsWithdrawalRestoresTheUsersModelOverrides(t *testing.T) {
	for _, test := range []struct {
		name, before string
		want         map[string]string
	}{
		{"none", `{"theme":"dark"}`, nil},
		{"the user's", `{"theme":"dark","modelOverrides":{"claude-opus-4-8":"my-opus"}}`, map[string]string{"claude-opus-4-8": "my-opus"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(test.before), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, false, overridesRuntime(), testExecutable(), ""); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err != nil {
				t.Fatal(err)
			}
			got, present := writtenOverrides(t, path)
			if present != (test.want != nil) || !maps.Equal(got, test.want) {
				t.Fatalf("after withdrawal modelOverrides = %v (present %t), want %v", got, present, test.want)
			}
			if _, err := os.Stat(path + adapter.StateSuffix); !os.IsNotExist(err) {
				t.Fatalf("the record remains after withdrawal: %v", err)
			}
		})
	}
}

// A map changed outside AIGW is managed drift: the next projection refuses
// and writes nothing.
func TestSettingsRefuseAModelOverridesEditOutsideAIGW(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := overridesRuntime()
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := []byte(strings.Replace(string(data), `"claude-code-fable-5-2"`, `"someone-elses-fable"`, 1))
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	next := runtime
	next.Endpoint = "https://next.test"
	_, err = ReconcileSettings(path, false, next, testExecutable(), runtime.UpstreamModel)
	if err == nil || !strings.Contains(err.Error(), "managed Claude settings changed") {
		t.Fatalf("an edited map = %v, want the refusal of an outside edit", err)
	}
	if current, readErr := os.ReadFile(path); readErr != nil || string(current) != string(edited) {
		t.Fatalf("the refused projection wrote the settings: %v", readErr)
	}
}
