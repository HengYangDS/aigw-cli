package cli_test

import (
	"aigw-cli/internal/prompt"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestProfileRenameInteractiveZeroArgsSortsChoicesAndUpdatesRoutes(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := profileRenameConfig()
	cfg.Profiles["alpha"] = configuration.Profile{Label: "Alpha", Account: "gateway", Client: configuration.ClientCodex, Model: "alpha"}
	cfg.Routes[configuration.ClientCodex] = "zeta-old"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "account-token"); err != nil {
		t.Fatal(err)
	}
	prompt := &fakePrompt{selected: "zeta-old", texts: []string{"zeta-new"}}
	app.Interactive = true
	app.Prompt = prompt

	if err := execute(t, app, "profile", "rename"); err != nil {
		t.Fatal(err)
	}

	if got, want := choiceValues(prompt.choices), []string{"alpha", "zeta-old"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("profile choices = %q, want %q", got, want)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes[configuration.ClientCodex] != "zeta-new" {
		t.Fatalf("routes after rename = %#v", got.Routes)
	}
	profile, ok := got.Profiles["zeta-new"]
	if !ok || profile.Label != "Zeta" || profile.Account != "gateway" {
		t.Fatalf("renamed profile = %#v, present = %v", profile, ok)
	}
	if _, ok := got.Profiles["zeta-old"]; ok {
		t.Fatal("old profile remains after rename")
	}
	if token, err := secretStore.Get("gateway"); err != nil || token != "account-token" {
		t.Fatalf("account token changed: %q, %v", token, err)
	}
	if secretExists(t, secretStore, "zeta-old") || secretExists(t, secretStore, "zeta-new") {
		t.Fatal("profile rename created profile-level secret slots")
	}
}

func TestProfileRenameInteractiveOneArgPromptsOnlyForTarget(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	if err := app.Config.Save(profileRenameConfig()); err != nil {
		t.Fatal(err)
	}
	prompt := &fakePrompt{texts: []string{"zeta-new"}}
	app.Interactive = true
	app.Prompt = prompt

	if err := execute(t, app, "profile", "rename", "zeta-old"); err != nil {
		t.Fatal(err)
	}

	if len(prompt.choices) != 0 {
		t.Fatalf("source selector was shown for an explicit source: %#v", prompt.choices)
	}
	if prompt.textCalls != 1 {
		t.Fatalf("target prompts = %d, want 1", prompt.textCalls)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Profiles["zeta-new"]; !ok {
		t.Fatalf("renamed profile missing: %#v", got.Profiles)
	}
}

func TestProfileRenameNonInteractiveRequiresBothIDs(t *testing.T) {
	for _, args := range [][]string{
		{"profile", "rename"},
		{"profile", "rename", "zeta-old"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			app, _, _, _ := testApp(t, "")
			if err := app.Config.Save(profileRenameConfig()); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}

			err = execute(t, app, args...)
			if err == nil || !strings.Contains(err.Error(), "non-interactive") {
				t.Fatalf("error = %v, want explicit non-interactive guidance", err)
			}
			after, readErr := os.ReadFile(app.Config.Path())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatal("missing rename arguments changed configuration")
			}
		})
	}
}

func TestProfileRenameDryRunJSONIsSecretFreeAndDoesNotWrite(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	cfg := profileRenameConfig()
	cfg.Routes[configuration.ClientCodex] = "zeta-old"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	const fixtureSecret = "fixture-profile-rename-secret"
	if err := secretStore.Set("gateway", fixtureSecret); err != nil {
		t.Fatal(err)
	}
	beforeConfig, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	beforeFiles := directoryNames(t, filepath.Dir(app.Config.Path()))
	out.Reset()

	if err := execute(t, app, "profile", "rename", "zeta-old", "zeta-new", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}

	var result struct {
		Resource           string            `json:"resource"`
		OldID              string            `json:"old_id"`
		NewID              string            `json:"new_id"`
		Status             string            `json:"status"`
		AffectedReferences []string          `json:"affected_references"`
		Actions            map[string]string `json:"actions"`
		ExternalTODOs      []string          `json:"external_todos"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, out.String())
	}
	if result.Resource != "profile" || result.OldID != "zeta-old" || result.NewID != "zeta-new" || result.Status != "planned" {
		t.Fatalf("rename result = %#v", result)
	}
	wantReferences := []string{"routes.codex"}
	if !reflect.DeepEqual(result.AffectedReferences, wantReferences) {
		t.Fatalf("affected references = %q, want %q", result.AffectedReferences, wantReferences)
	}
	for _, key := range []string{"configuration", "api_token", "account_probe", "authentication", "backup"} {
		if result.Actions[key] == "" {
			t.Fatalf("missing %q action in %#v", key, result.Actions)
		}
	}
	if result.Actions["api_token"] != "unchanged" || result.Actions["account_probe"] != "unchanged" {
		t.Fatalf("profile rename credential actions = %#v", result.Actions)
	}
	if result.ExternalTODOs == nil || len(result.ExternalTODOs) != 0 {
		t.Fatalf("external todos = %#v, want an empty JSON array", result.ExternalTODOs)
	}
	if strings.Contains(out.String(), fixtureSecret) || strings.Contains(out.String(), app.Config.Path()) {
		t.Fatalf("dry-run output leaked a secret or local path: %s", out.String())
	}
	afterConfig, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterConfig, beforeConfig) {
		t.Fatal("dry-run changed configuration bytes")
	}
	if got := directoryNames(t, filepath.Dir(app.Config.Path())); !reflect.DeepEqual(got, beforeFiles) {
		t.Fatalf("dry-run changed config directory entries: before %q, after %q", beforeFiles, got)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("dry-run invoked a client: %#v", runner.plans)
	}
	if token, err := secretStore.Get("gateway"); err != nil || token != fixtureSecret {
		t.Fatalf("dry-run changed account token: %q, %v", token, err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Profiles["zeta-old"]; !ok {
		t.Fatalf("dry-run removed source profile: %#v", got.Profiles)
	}
	if _, ok := got.Profiles["zeta-new"]; ok {
		t.Fatalf("dry-run created target profile: %#v", got.Profiles)
	}
}

func TestProfileRenameRefusesInvalidOrConflictingTargetWithoutMutation(t *testing.T) {
	for _, target := range []string{"alpha", "Invalid Target"} {
		t.Run(target, func(t *testing.T) {
			app, _, _, _ := testApp(t, "")
			cfg := profileRenameConfig()
			cfg.Profiles["alpha"] = configuration.Profile{Label: "Alpha", Account: "gateway", Client: configuration.ClientCodex, Model: "alpha-model"}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}

			if err := execute(t, app, "profile", "rename", "zeta-old", target, "--dry-run"); err == nil {
				t.Fatalf("rename to %q unexpectedly succeeded", target)
			}
			after, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("refused target %q changed configuration", target)
			}
		})
	}
}

func profileRenameConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["zeta-old"] = configuration.Profile{Label: "Zeta", Purpose: "Keep this label and purpose", Account: "gateway", Client: configuration.ClientCodex, Model: "zeta-model"}
	cfg.Routes[configuration.ClientCodex] = "zeta-old"
	return cfg
}

func choiceValues(choices []prompt.Choice) []string {
	values := make([]string, 0, len(choices))
	for _, choice := range choices {
		values = append(values, choice.Value)
	}
	return values
}

func directoryNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
