package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestClaudeModelDriftRecoveryThroughPublicCommands(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		model   string
		preview bool
	}{
		{"sync", []string{"sync"}, `"claude-test"`, true},
		{"repair", []string{"repair"}, `"claude-test"`, true},
		{"use", []string{"use", "next"}, `"claude-next"`, false},
		{"repeat-use", []string{"use", "one"}, `"claude-test"`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, credentials, runner, _ := testApp(t, "")
			saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
			profile := cfg.Profiles["one"]
			profile.Model = "claude-next"
			cfg.Profiles["next"] = profile
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			synchronizeClaudeProjection(t, app, cfg)
			data, err := os.ReadFile(app.ClaudeSettingsPath)
			if err != nil {
				t.Fatal(err)
			}
			var settings map[string]json.RawMessage
			if err := json.Unmarshal(data, &settings); err != nil {
				t.Fatal(err)
			}
			delete(settings, "model")
			settings["theme"] = json.RawMessage(`"dark"`)
			data, err = json.Marshal(settings)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(app.ClaudeSettingsPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if test.preview {
				assertClaudeModelRepairPreview(t, app, out, test.name, data)
			}
			if err := cli.Execute(app, test.args); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(app.ClaudeSettingsPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &settings); err != nil {
				t.Fatal(err)
			}
			if string(settings["model"]) != test.model || string(settings["theme"]) != `"dark"` {
				t.Fatalf("settings=%s", data)
			}
			if len(runner.plans) != 0 {
				t.Fatal("configuration recovery started a client")
			}
		})
	}
}

func assertClaudeModelRepairPreview(t *testing.T, app *cli.App, out *bytes.Buffer, command string, settings []byte) {
	t.Helper()
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(app.ClaudeSettingsPath + ".aigw-state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{command, "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Targets     []client.ProjectionPlan `json:"targets"`
		Projections []client.ProjectionPlan `json:"projections"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if command == "repair" {
		preview.Targets = preview.Projections
	}
	if len(preview.Targets) != 1 || preview.Targets[0].Client != configuration.ClientClaude || preview.Targets[0].Action != "project" {
		t.Fatalf("preview=%+v", preview)
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("preview changed configuration")
	}
	if current, err := os.ReadFile(app.ClaudeSettingsPath); err != nil || !bytes.Equal(current, settings) {
		t.Fatal("preview changed user settings")
	}
	if current, err := os.ReadFile(app.ClaudeSettingsPath + ".aigw-state.json"); err != nil || !bytes.Equal(current, state) {
		t.Fatal("preview changed ownership state")
	}
}

func TestRecoveryJSONSeparatesPreviewFromAppliedProjection(t *testing.T) {
	for _, test := range []struct {
		args       []string
		dryRun     bool
		nextAction string
	}{
		{[]string{"sync", "--json", "--dry-run"}, true, "aigw sync"},
		{[]string{"repair", "--json", "--dry-run"}, true, "aigw repair"},
		{[]string{"sync", "--json"}, false, "aigw check"},
		{[]string{"repair", "--json"}, false, "aigw check"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			app, out, credentials, _, _ := testApp(t, "")
			saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: executableFixture(t, "claude")}}}
			before, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, test.args); err != nil {
				t.Fatal(err)
			}
			var result struct {
				DryRun     bool   `json:"dry_run"`
				NextAction string `json:"next_action"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("expected one JSON result: %v\n%s", err, out)
			}
			if result.DryRun != test.dryRun || result.NextAction != test.nextAction {
				t.Fatalf("result = %+v, want dry_run=%t, next_action=%q", result, test.dryRun, test.nextAction)
			}
			after, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}
			if test.dryRun {
				if !bytes.Equal(before, after) {
					t.Fatal("preview changed configuration")
				}
				if _, err := os.Stat(app.ClaudeSettingsPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("preview created client settings: %v", err)
				}
				return
			}
			settings, err := os.ReadFile(app.ClaudeSettingsPath)
			if err != nil || !json.Valid(settings) {
				t.Fatalf("applied projection = %q, %v", settings, err)
			}
			if bytes.Equal(before, after) {
				t.Fatal("applied discovery was not persisted")
			}
		})
	}
}

func TestRecoveryJSONOutputFailurePreservesCommittedProjection(t *testing.T) {
	for _, command := range []string{"sync", "repair"} {
		t.Run(command, func(t *testing.T) {
			app, _, credentials, _, _ := testApp(t, "")
			saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: executableFixture(t, "claude")}}}
			want := errors.New("output is unavailable")
			app.Out = failingOutput{err: want}
			if err := cli.Execute(app, []string{command, "--json"}); !errors.Is(err, want) {
				t.Fatalf("output error = %v, want %v", err, want)
			}
			cfg, err := app.Config.Load()
			if err != nil || !cfg.Adapters[configuration.ClientClaude].Enabled {
				t.Fatalf("committed adapter = %+v, %v", cfg.Adapters[configuration.ClientClaude], err)
			}
			settings, err := os.ReadFile(app.ClaudeSettingsPath)
			if err != nil || !json.Valid(settings) {
				t.Fatalf("committed projection = %q, %v", settings, err)
			}
		})
	}
}
