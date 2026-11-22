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
		UpstreamModel:  "claude-code-opus-5-5",
		FamilyVariants: map[string]string{"claude-opus-5-5": "claude-code-opus-5-5", "claude-fable-5-2": "claude-code-fable-5-2", "gpt-6.2-sol": "codex-gpt-6.2-sol"},
	}
}

// opusOnlyRuntime is the same selection over a provider that serves the
// selected Anthropic model under its own ID and no other.
func opusOnlyRuntime() domain.Runtime {
	runtime := overridesRuntime()
	runtime.FamilyVariants = map[string]string{"claude-opus-5-5": "claude-code-opus-5-5", "gpt-6.2-sol": "codex-gpt-6.2-sol"}
	return runtime
}

// anotherChannelRuntime is the same selection over the same provider after a
// channel of another model changed.
func anotherChannelRuntime() domain.Runtime {
	runtime := overridesRuntime()
	runtime.FamilyVariants = map[string]string{"claude-opus-5-5": "claude-code-opus-5-5", "claude-fable-5-2": "claude-code-fable-5-2b"}
	return runtime
}

// withoutMapRuntime is the same selection over a provider that serves no
// Anthropic model under another ID.
func withoutMapRuntime() domain.Runtime {
	runtime := overridesRuntime()
	runtime.FamilyVariants = map[string]string{"gpt-6.2-sol": "codex-gpt-6.2-sol"}
	return runtime
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

// The table's rows one and three: without an entry and without a map the key
// is not AIGW's and stays; with a projection that writes the map the same
// projection again writes nothing, and the record's entry is what makes the
// map AIGW's.
func TestSettingsProjectTheProvidersModelOverrides(t *testing.T) {
	t.Run("row two and row three", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		runtime := overridesRuntime()
		if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"claude-opus-5-5": "claude-code-opus-5-5", "claude-fable-5-2": "claude-code-fable-5-2"}
		if got, present := writtenOverrides(t, path); !present || !maps.Equal(got, want) {
			t.Fatalf("modelOverrides = %v (present %t), want %v: a family that is no Anthropic model ID stays out", got, present, want)
		}
		requireOwnedMapDigest(t, path)
		plan, err := PlanSettings(path, false, runtime, testExecutable(), runtime.UpstreamModel)
		if err != nil || plan.Action != SettingsActionAlreadyConverged {
			t.Fatalf("the projected map plans %q, %v; want already converged", plan.Action, err)
		}
		if _, err := InspectSettings(path, runtime, testExecutable()); err != nil {
			t.Fatalf("inspection of the projected map: %v", err)
		}
	})

	t.Run("row four", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		if _, err := ReconcileSettings(path, false, overridesRuntime(), testExecutable(), ""); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileSettings(path, false, withoutMapRuntime(), testExecutable(), overridesRuntime().UpstreamModel); err != nil {
			t.Fatal(err)
		}
		if got, present := writtenOverrides(t, path); present {
			t.Fatalf("a selection without a map left modelOverrides %v", got)
		}
		if _, present := readOwnedKeysFile(t, path)["modelOverrides"]; present {
			t.Fatal("a selection without a map kept the entry once it gave the key back")
		}
	})

	t.Run("row one", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(path, []byte(`{"modelOverrides":{"claude-opus-4-8":"my-opus"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileSettings(path, false, withoutMapRuntime(), testExecutable(), ""); err != nil {
			t.Fatal(err)
		}
		if got, present := writtenOverrides(t, path); !present || !maps.Equal(got, map[string]string{"claude-opus-4-8": "my-opus"}) {
			t.Fatalf("a selection without a map changed the user's modelOverrides: %v (present %t)", got, present)
		}
		if _, err := os.Stat(ownedKeysPath(path)); !os.IsNotExist(err) {
			t.Fatalf("a key AIGW did not write has an entry: %v", err)
		}
	})
}

// The table's rows four and five at a withdrawal: the user's map comes back
// when AIGW wrote over it, and settings that held none hold none afterwards.
func TestSettingsWithdrawalRestoresTheUsersModelOverrides(t *testing.T) {
	for _, test := range []struct {
		name, before string
		want         map[string]string
	}{
		{"row four: the user's map returns", `{"theme":"dark","modelOverrides":{"claude-opus-4-8":"my-opus"}}`, map[string]string{"claude-opus-4-8": "my-opus"}},
		{"row four: a map AIGW wrote alone leaves none", `{"theme":"dark"}`, nil},
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

// The table's row five: a map changed outside AIGW is managed drift, and the
// next projection refuses and writes nothing.
func TestSettingsRefuseAModelOverridesEditOutsideAIGW(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := overridesRuntime()
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	edited := []byte(strings.Replace(string(readFileBytes(t, path)), `"claude-code-fable-5-2"`, `"someone-elses-fable"`, 1))
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	keys := readFileBytes(t, ownedKeysPath(path))
	next := runtime
	next.Endpoint = "https://next.test"
	_, err := ReconcileSettings(path, false, next, testExecutable(), runtime.UpstreamModel)
	if err == nil || !strings.Contains(err.Error(), "managed Claude settings changed") {
		t.Fatalf("an edited map = %v, want the refusal of an outside edit", err)
	}
	if got := readFileBytes(t, path); string(got) != string(edited) {
		t.Fatalf("the refused projection wrote the settings: %s", got)
	}
	if got := readFileBytes(t, ownedKeysPath(path)); string(got) != string(keys) {
		t.Fatalf("the refused projection wrote the record: %s", got)
	}
}

// A change of the map alone, here a channel that the provider gains, changes
// Claude Code's projection, so the command that makes it rewrites the
// settings.
func TestClaudeProjectionChangesWhenOnlyTheModelOverridesChange(t *testing.T) {
	before := domain.NewConfig(testAdmission())
	before.Providers["suhe"] = domain.Provider{Label: "Suhe", Endpoints: domain.Endpoints{Anthropic: "https://suhe.test"}}
	opus := qualifiedModel("", "suhe", "claude-code-opus-5-5", domain.ProtocolAnthropic)
	opus.Family = "claude-opus-5-5"
	before.Models[opus.Ref()] = opus
	before.Clients[ID] = domain.ClientBinding{Model: opus.Ref(), Enabled: true, Executable: "/usr/bin/claude"}
	if (claudeAdapter{}).ProjectionChanged(before, before.Clone()) {
		t.Fatal("an unchanged configuration changes the projection")
	}
	after := before.Clone()
	fable := qualifiedModel("", "suhe", "claude-code-fable-5-2", domain.ProtocolAnthropic)
	fable.Family = "claude-fable-5-2"
	after.Models[fable.Ref()] = fable
	if !(claudeAdapter{}).ProjectionChanged(before, after) {
		t.Fatal("a channel that the provider gained did not change Claude Code's projection")
	}
}
