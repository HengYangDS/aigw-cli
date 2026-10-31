package cli_test

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestClaudeNativeModelPreferenceThroughPublicCommands(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		model   string
		preview bool
	}{
		{"sync", []string{"sync"}, "", true},
		{"repair", []string{"repair"}, "", true},
		{"use", []string{"use", "--for", "claude", "next"}, `"claude-next"`, false},
		{"repeat-use", []string{"use", "--for", "claude", "one"}, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, credentials, runner, _ := testApp(t, "")
			saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
			route := cfg.Routes["one"]
			route.Model = "claude-next"
			route.UpstreamModel = "claude-next"
			cfg.Routes["next"] = route
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			synchronizeClaudeProjection(t, app, cfg)
			data, err := os.ReadFile(claudeSettingsPath(t, app.Host))
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
			if err := os.WriteFile(claudeSettingsPath(t, app.Host), data, 0o600); err != nil {
				t.Fatal(err)
			}
			settingsBefore := append([]byte(nil), data...)
			if test.preview {
				assertClaudeNativePreferencePreview(t, app, out, test.name, data)
			}
			if err := cli.Execute(app, test.args); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(claudeSettingsPath(t, app.Host))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &settings); err != nil {
				t.Fatal(err)
			}
			if string(settings["model"]) != test.model || string(settings["theme"]) != `"dark"` {
				t.Fatalf("settings=%s", data)
			}
			if test.model == "" && !bytes.Equal(data, settingsBefore) {
				t.Fatal("unchanged Route rewrote native model preference")
			}
			if len(runner.plans) != 0 {
				t.Fatal("configuration recovery started a client")
			}
		})
	}
}

func assertClaudeNativePreferencePreview(t *testing.T, app *cli.App, out *bytes.Buffer, command string, settings []byte) {
	t.Helper()
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(claudeSettingsPath(t, app.Host) + ".aigw-state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{command, "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	// sync and repair name the client files alike.
	var preview struct {
		Projections []adapter.ProjectionPlan `json:"projections"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Projections) != 1 || preview.Projections[0].Client != claude.ID || preview.Projections[0].Action != "already_converged" {
		t.Fatalf("preview=%+v", preview)
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("preview changed configuration")
	}
	if current, err := os.ReadFile(claudeSettingsPath(t, app.Host)); err != nil || !bytes.Equal(current, settings) {
		t.Fatal("preview changed user settings")
	}
	if current, err := os.ReadFile(claudeSettingsPath(t, app.Host) + ".aigw-state.json"); err != nil || !bytes.Equal(current, state) {
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
			saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(claude.ID, true, "", nil)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
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
				if _, err := os.Stat(claudeSettingsPath(t, app.Host)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("preview created client settings: %v", err)
				}
				return
			}
			settings, err := os.ReadFile(claudeSettingsPath(t, app.Host))
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
			saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(claude.ID, true, "", nil)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
			want := errors.New("output is unavailable")
			app.Out = failingOutput{err: want}
			if err := cli.Execute(app, []string{command, "--json"}); !errors.Is(err, want) {
				t.Fatalf("output error = %v, want %v", err, want)
			}
			cfg, err = app.Config.Load()
			if err != nil || !cfg.Clients[claude.ID].Enabled {
				t.Fatalf("committed adapter = %+v, %v", cfg.Clients[claude.ID], err)
			}
			settings, err := os.ReadFile(claudeSettingsPath(t, app.Host))
			if err != nil || !json.Valid(settings) {
				t.Fatalf("committed projection = %q, %v", settings, err)
			}
		})
	}
}
