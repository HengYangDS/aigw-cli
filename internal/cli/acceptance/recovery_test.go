package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
	"aigw-cli/internal/transaction"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
			saveCommandRoute(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
			route := cfg.Routes["one"]
			route.Model = "claude-next"
			route.UpstreamModel = "claude-next"
			cfg.Routes["next"] = route
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
			settingsBefore := append([]byte(nil), data...)
			if test.preview {
				assertClaudeNativePreferencePreview(t, app, out, test.name, data)
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
	if len(preview.Targets) != 1 || preview.Targets[0].Client != configuration.ClientClaude || preview.Targets[0].Action != "already-converged" {
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
			saveCommandRoute(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(configuration.ClientClaude, true, "", nil)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
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
			saveCommandRoute(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(configuration.ClientClaude, true, "", nil)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := credentials.Set("one", "test-token"); err != nil {
				t.Fatal(err)
			}
			app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: executableFixture(t, "claude")}}}
			want := errors.New("output is unavailable")
			app.Out = failingOutput{err: want}
			if err := cli.Execute(app, []string{command, "--json"}); !errors.Is(err, want) {
				t.Fatalf("output error = %v, want %v", err, want)
			}
			cfg, err = app.Config.Load()
			if err != nil || !cfg.Clients[configuration.ClientClaude].Enabled {
				t.Fatalf("committed adapter = %+v, %v", cfg.Clients[configuration.ClientClaude], err)
			}
			settings, err := os.ReadFile(app.ClaudeSettingsPath)
			if err != nil || !json.Valid(settings) {
				t.Fatalf("committed projection = %q, %v", settings, err)
			}
		})
	}
}

func TestForwardingPreviewPreservesConfigurationAndNativeDirectories(t *testing.T) {
	for _, mode := range []string{"forwarding", "direct"} {
		t.Run(mode, func(t *testing.T) {
			app, out, secretStore, runner, httpClient := testApp(t, "")
			root := t.TempDir()
			app.Config = configuration.NewStore(filepath.Join(root, "aigw", "configuration.toml"))
			target := filepath.Join(root, "codex", "config.toml")
			executable := executableFixture(t, "codex")
			cfg := configuration.NewConfig()
			addAccountRoute(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
			cfg.SetSelectedRoute(configuration.ClientCodex, "one")
			cfg.SetClientActivation(configuration.ClientCodex, true, executable, []string{target})
			binding := cfg.Clients[configuration.ClientCodex]
			binding.Protocol, binding.CredentialCommand = configuration.ProtocolOpenAIResponses, app.Executable
			cfg.Clients[configuration.ClientCodex] = binding
			args := []string{"use", "--for", "codex", "one", "--dry-run", "--json"}
			if mode == "direct" {
				if err := cfg.SetForwardingEndpoint(configuration.ClientCodex, "http://127.0.0.1:8792/v1"); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--direct")
			} else {
				args = append(args, "--forwarding-endpoint", "http://127.0.0.1:8792/v1")
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			writeFile(t, target, []byte("# Native preference\nmodel_reasoning_effort = \"medium\"\n"), 0o600)
			credentials := &recordingCredentialStore[string]{backend: secretStore}
			app.Secrets = credentials
			app.Discovery = fakeDiscovery{result: discovery.Result{
				Executables: map[string]string{configuration.ClientCodex: executable},
				Surfaces:    []discovery.Surface{{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target, AutoManaged: true, Present: true}},
			}}
			before, err := app.Config.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			nativeBefore, err := transaction.CaptureFileSnapshot(target)
			if err != nil {
				t.Fatal(err)
			}
			directories := map[string][]string{}
			for _, path := range []string{filepath.Dir(app.Config.Path()), filepath.Dir(target)} {
				directories[path] = directoryNames(t, path)
			}
			if err := cli.Execute(app, args); err != nil {
				t.Fatalf("preview: %v\n%s", err, out)
			}
			for path, names := range directories {
				if got := directoryNames(t, path); !reflect.DeepEqual(got, names) {
					t.Fatalf("preview changed directory %s: got %v, want %v", path, got, names)
				}
			}
			after, err := app.Config.CaptureSnapshot()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("preview changed configuration or recovery files: %v", err)
			}
			nativeAfter, err := transaction.CaptureFileSnapshot(target)
			if err != nil || !reflect.DeepEqual(nativeBefore, nativeAfter) {
				t.Fatalf("preview changed native settings or mode: %v", err)
			}
			if len(credentials.getCalls)+len(credentials.existsCalls)+len(credentials.setCalls)+len(credentials.deleteCalls)+len(runner.plans)+httpClient.calls != 0 {
				t.Fatal("preview accessed credentials, a client, or the network")
			}
		})
	}
}

func TestForwardingCommandsPreserveEnabledHermesAndOriginalReader(t *testing.T) {
	app, out, credentials, runner, _ := testApp(t, "")
	root := t.TempDir()
	target := filepath.Join(root, "codex", "config.toml")
	hermesTarget := filepath.Join(root, "hermes", "config.yaml")
	reader := app.Executable
	cfg := configuration.NewConfig()
	addAccountRoute(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
	for _, pair := range []struct{ client, executable, target string }{
		{configuration.ClientCodex, executableFixture(t, "codex"), target},
		{configuration.ClientHermes, executableFixture(t, "hermes"), hermesTarget},
	} {
		cfg.SetSelectedRoute(pair.client, "one")
		cfg.SetClientActivation(pair.client, true, pair.executable, []string{pair.target})
		binding := cfg.Clients[pair.client]
		binding.Protocol, binding.CredentialCommand = configuration.ProtocolOpenAIResponses, reader
		cfg.Clients[pair.client] = binding
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := credentials.Set("one", "synthetic"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("# Native preference\nmodel_reasoning_effort = \"medium\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(hermesTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hermesTarget, []byte("# User preference\nterminal:\n  theme: dark\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{
			configuration.ClientCodex:  cfg.Clients[configuration.ClientCodex].Executable,
			configuration.ClientHermes: cfg.Clients[configuration.ClientHermes].Executable,
		},
		Surfaces: []discovery.Surface{
			{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target, AutoManaged: true, Present: true},
			{ID: "hermes-home-default", Authority: "aigw", Product: "Hermes", ConfigPath: hermesTarget, Present: true},
		},
	}}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	modes := map[string]os.FileMode{}
	for _, path := range []string{hermesTarget, hermesTarget + ".aigw-state.json"} {
		files[path] = readFile(t, path)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		modes[path] = info.Mode().Perm()
	}
	for _, step := range []struct {
		args     []string
		endpoint string
	}{
		{[]string{"use", "--for", "codex", "one", "--forwarding-endpoint", "http://127.0.0.1:8792/v1"}, "http://127.0.0.1:8792/v1"},
		{[]string{"use", "--for", "codex", "one", "--direct"}, "https://one.test/v1"},
		{[]string{"rollback", "--last-change"}, "http://127.0.0.1:8792/v1"},
		{[]string{"use", "--for", "codex", "one", "--direct"}, "https://one.test/v1"},
	} {
		out.Reset()
		if err := cli.Execute(app, step.args); err != nil {
			t.Fatalf("%v: %v\n%s", step.args, err, out.String())
		}
		requireForwardedCodexProjection(t, app.Config, before, target, reader, step.endpoint)
		for path, data := range files {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != modes[path] || !bytes.Equal(readFile(t, path), data) {
				t.Fatalf("%v changed enabled Hermes files or modes: %v", step.args, err)
			}
		}
		if len(runner.plans) != 0 {
			t.Fatal("projection selection started a client")
		}
	}
}

func requireForwardedCodexProjection(t *testing.T, store configuration.Store, before configuration.Config, target, reader, endpoint string) {
	t.Helper()
	current, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := current.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil || selected.Endpoint != endpoint {
		t.Fatalf("selected endpoint = %q, want %q: %v", selected.Endpoint, endpoint, err)
	}
	data := readFile(t, target)
	if !bytes.Contains(data, []byte(endpoint)) || !bytes.Contains(data, []byte(reader)) ||
		!bytes.Contains(data, []byte("# Native preference")) || !bytes.Contains(data, []byte("medium")) {
		t.Fatal("selection changed endpoint, reader or native preferences")
	}
	if !reflect.DeepEqual(current.Accounts, before.Accounts) ||
		!reflect.DeepEqual(current.Clients[configuration.ClientHermes], before.Clients[configuration.ClientHermes]) {
		t.Fatal("selection changed Account or enabled Hermes binding")
	}
}
