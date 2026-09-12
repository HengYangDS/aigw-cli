package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSyncHumanPreviewHandlesDisabledAndEnabledAdapters(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"sync", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "No client configuration needs changing") {
			t.Fatalf("output = %q", out.String())
		}
	})

	t.Run("enabled", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		target := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := configuration.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		cfg.Routes[configuration.ClientCodex] = "one"
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := cli.Execute(app, []string{"sync", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
		var renderedTargets []string
		for line := range strings.SplitSeq(out.String(), "\n") {
			line = strings.TrimSpace(line)
			if before, ok := strings.CutSuffix(line, " initial-project"); ok {
				renderedTargets = append(renderedTargets, strings.TrimSpace(before))
			}
		}
		if len(renderedTargets) != 1 {
			t.Fatalf("initial-project rows = %#v, output = %q", renderedTargets, out.String())
		}
		assertSameExistingPath(t, renderedTargets[0], target)
	})
}

func TestCodexSyncReconcilesEachConfiguredHomeWithoutLoggingIn(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	dir := t.TempDir()
	targets := []string{filepath.Join(dir, "one", "configuration.toml"), filepath.Join(dir, "two", "configuration.toml")}
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}, configuration.ClientCodex, "team-model")
	cfg.Routes[configuration.ClientCodex] = "team"
	cfg.Adapters["codex"] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex-real", Targets: targets}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync must not start credential binding plans: %#v", runner.plans)
	}
	for _, target := range targets {
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "AIGW managed provider") {
			t.Fatalf("sync did not reconcile %s:\n%s", target, data)
		}
	}
}

func TestSyncReconcilesCodexConfigWithoutRebindingCredentials(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model = "gpt-test" # managed by AIGW`) {
		t.Fatalf("sync did not reconcile Codex config:\n%s", data)
	}
}

func TestSyncAndCheckTreatDirectAndLoopbackEndpointsAsOrdinaryAccountChoices(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{name: "direct HTTPS", endpoint: "https://provider.test/v1"},
		{name: "explicit loopback", endpoint: "http://127.0.0.1:48721/v1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, runner, httpClient := testApp(t, "")
			target := filepath.Join(t.TempDir(), "configuration.toml")
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := configuration.NewConfig()
			addAccountProfile(&cfg, "codex", "provider", "Provider", configuration.Endpoints{OpenAIResponses: test.endpoint}, configuration.ClientCodex, "gpt-test")
			cfg.Routes[configuration.ClientCodex] = "codex"
			cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{target}}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("provider", "test-token"); err != nil {
				t.Fatal(err)
			}

			if err := cli.Execute(app, []string{"sync"}); err != nil {
				t.Fatalf("sync: %v", err)
			}
			projection, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(projection), `base_url = "`+test.endpoint+`"`) {
				t.Fatalf("projection does not contain selected Account endpoint:\n%s", projection)
			}
			if len(runner.plans) != 0 {
				t.Fatalf("sync started an external process: %#v", runner.plans)
			}

			var requestURL string
			httpClient.handler = func(request *http.Request) (*http.Response, error) {
				requestURL = request.URL.String()
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: request}, nil
			}
			out.Reset()
			if err := cli.Execute(app, []string{"check"}); err != nil {
				t.Fatalf("check: %v\n%s", err, out.String())
			}
			if want := strings.TrimRight(test.endpoint, "/") + "/models"; requestURL != want {
				t.Fatalf("diagnostic URL = %q, want %q", requestURL, want)
			}
			if len(runner.plans) != 0 {
				t.Fatalf("check started an external process: %#v", runner.plans)
			}
		})
	}
}

func TestSyncUsesSharedCodexHomeAndOfficialClaudeSettingsWithoutTouchingClientState(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	for _, name := range []string{"aigw", configuration.ClientClaude, configuration.ClientCodex} {
		writeFile(t, filepath.Join(bin, executableName(name)), []byte("native executable fixture"), 0o755)
	}

	codexHome := filepath.Join(home, ".codex")
	codexTarget := filepath.Join(codexHome, "config.toml")
	writeFile(t, codexTarget, []byte("model_provider = \"native\"\ndesktop_feature = true\n"), 0o600)
	preserved := map[string][]byte{
		filepath.Join(codexHome, "sessions", "existing.jsonl"): []byte("{\"model\":\"gpt-existing\"}\n"),
		filepath.Join(codexHome, "state_5.sqlite"):             []byte("SQLite format 3\x00existing state"),
		filepath.Join(codexHome, "desktop-settings.json"):      []byte("{\"theme\":\"system\"}\n"),
		filepath.Join(home, ".zshrc"):                          []byte("export TEAM_VALUE=kept\n"),
	}
	for path, data := range preserved {
		writeFile(t, path, data, 0o600)
	}
	claudeSettings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, claudeSettings, []byte(`{"theme":"dark","permissions":{"allow":["Read"]},"env":{"TEAM_VALUE":"kept"}}`), 0o600)

	app, _, secretStore, runner, _ := testApp(t, "")
	app.Executable = filepath.Join(bin, executableName("aigw"))
	app.ClaudeSettingsPath = claudeSettings
	app.Discovery = client.NewDiscoverer(client.DefaultRegistry(), discovery.System{GOOS: runtime.GOOS, Home: home, Path: bin})
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	cfg.Profiles = map[string]configuration.Profile{
		"claude": {Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-team"},
		"codex":  {Label: "Codex", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-team"},
	}
	cfg.Routes = configuration.Routes{configuration.ClientClaude: "claude", configuration.ClientCodex: "codex"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "plaintext-token-must-not-be-projected"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started a client or credential command: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(after.Profiles, cfg.Profiles) || !maps.Equal(after.Routes, cfg.Routes) {
		t.Fatalf("sync changed Profile or Route authority: profiles=%#v routes=%#v", after.Profiles, after.Routes)
	}
	adapter := after.Adapters[configuration.ClientCodex]
	if !adapter.Enabled || len(adapter.Targets) != 1 || adapter.Targets[0] != codexTarget {
		t.Fatalf("Codex did not use the single discovered shared home: %#v", adapter)
	}
	codexConfig := readFile(t, codexTarget)
	if !strings.Contains(string(codexConfig), "desktop_feature = true") || !strings.Contains(string(codexConfig), `model = "gpt-team" # managed by AIGW`) {
		t.Fatalf("Codex projection did not preserve foreign configuration:\n%s", codexConfig)
	}
	if strings.Contains(string(codexConfig), "plaintext-token-must-not-be-projected") {
		t.Fatal("Codex projection contains the Account Token")
	}
	for path, want := range preserved {
		if got := readFile(t, path); !bytes.Equal(got, want) {
			t.Fatalf("client-owned state changed at %s: %q", path, got)
		}
	}
	var settings map[string]any
	data := readFile(t, claudeSettings)
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	helper, _ := settings["apiKeyHelper"].(string)
	delete(settings, "apiKeyHelper")
	wantSettings := map[string]any{
		"model": "claude-team", "theme": "dark",
		"permissions": map[string]any{"allow": []any{"Read"}},
		"env":         map[string]any{"TEAM_VALUE": "kept", "ANTHROPIC_BASE_URL": "https://gateway.test"},
	}
	if !reflect.DeepEqual(settings, wantSettings) {
		t.Fatalf("Claude settings = %#v, want %#v", settings, wantSettings)
	}
	projection, err := after.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(helper, app.Executable) || !strings.HasSuffix(helper, " credential claude "+projection.CredentialProjectionFingerprint(configuration.ClientClaude)) {
		t.Fatalf("Claude helper is not the absolute AIGW credential command: %q", helper)
	}
	if strings.Contains(string(data), "plaintext-token-must-not-be-projected") {
		t.Fatalf("Claude settings contain credential material: %s", data)
	}
}

func TestSyncRefreshesTheClaudeHelperAfterAIGWMoves(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	app.ClaudeSettingsPath = settingsPath
	claudeExecutable := executableFixture(t, configuration.ClientClaude)
	oldAIGWExecutable := executableFixture(t, "aigw-old")
	newAIGWExecutable := executableFixture(t, "aigw-new")
	app.Executable = oldAIGWExecutable
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{
		configuration.ClientClaude: claudeExecutable,
	}}}

	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "gateway", "Gateway", configuration.Endpoints{Anthropic: "https://gateway.test"}, configuration.ClientClaude, "claude-team")
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "token-must-not-be-projected"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}

	app.Executable = newAIGWExecutable
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after AIGW moved: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started a client or credential command: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(after.Profiles, cfg.Profiles) || !maps.Equal(after.Routes, cfg.Routes) {
		t.Fatalf("sync changed Profile or Route authority: profiles=%#v routes=%#v", after.Profiles, after.Routes)
	}
	if after.Adapters[configuration.ClientClaude].Executable != claudeExecutable {
		t.Fatalf("sync changed the Claude executable: %#v", after.Adapters[configuration.ClientClaude])
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(settings.APIKeyHelper, newAIGWExecutable) || strings.Contains(settings.APIKeyHelper, oldAIGWExecutable) {
		t.Fatalf("Claude helper did not follow the installed AIGW executable: %q", settings.APIKeyHelper)
	}
	if strings.Contains(string(data), "token-must-not-be-projected") {
		t.Fatalf("Claude settings contain credential material: %s", data)
	}
}

func TestSyncSurfacesCredentialObservationFailure(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: want}

	if err := cli.Execute(app, []string{"sync"}); err == nil || err.Error() != "Synchronization prerequisites are unavailable" || !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	for _, expected := range []string{
		"Synchronization prerequisites are unavailable",
		"AIGW could not determine which selected Routes can be projected with the currently available clients and credentials.",
		"Configuration and client projections remain unchanged.",
		"aigw doctor",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("output missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), want.Error()) {
		t.Fatalf("output exposes implementation error:\n%s", out.String())
	}
}

func TestSyncDryRunReportsEveryTargetWithoutMutatingProjectionOrCredentials(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	dir := t.TempDir()
	first := filepath.Join(dir, "first.toml")
	second := filepath.Join(dir, "second.toml")
	claudeSettings := filepath.Join(dir, "settings.json")
	app.ClaudeSettingsPath = claudeSettings
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: "/usr/local/bin/claude"}}}
	for _, target := range []string{first, second} {
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "http://127.0.0.1:8791/v1", Anthropic: "https://gateway.test"}}
	cfg.Profiles["terra"] = configuration.Profile{Label: "GPT-5.6 Terra", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-5.6-terra"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "terra"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{first, second}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "dry-run-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
		t.Fatalf("sync --dry-run --json error = %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("dry-run started credential binding plans: %#v", runner.plans)
	}
	for _, target := range []string{first, second} {
		data, err := os.ReadFile(target)
		if err != nil || string(data) != "model_provider = \"native\"\n" {
			t.Fatalf("dry-run mutated %s: %q, %v", target, data, err)
		}
		if _, err := os.Stat(target + ".aigw-state.json"); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote sidecar %s: %v", target, err)
		}
	}
	if _, err := os.Stat(claudeSettings); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote Claude settings %s: %v", claudeSettings, err)
	}
	if _, err := os.Stat(claudeSettings + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote Claude settings state %s: %v", claudeSettings, err)
	}
	var preview struct {
		DryRun  bool `json:"dry_run"`
		Targets []struct {
			Client string `json:"client"`
			Target string `json:"target"`
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("decode sync dry-run JSON: %v\n%s", err, out.String())
	}
	wantTargets := []string{claudeSettings, first, second}
	wantClients := []string{configuration.ClientClaude, configuration.ClientCodex, configuration.ClientCodex}
	wantActions := []string{"project", "initial-project", "initial-project"}
	if !preview.DryRun || len(preview.Targets) != len(wantTargets) {
		t.Fatalf("sync dry-run preview = %#v", preview)
	}
	for index, want := range wantTargets {
		if preview.Targets[index].Client != wantClients[index] {
			t.Fatalf("target %d client = %q, want %q", index, preview.Targets[index].Client, wantClients[index])
		}
		if preview.Targets[index].Action != wantActions[index] {
			t.Fatalf("target %d action = %q, want %q", index, preview.Targets[index].Action, wantActions[index])
		}
		if wantClients[index] == configuration.ClientCodex {
			assertSameExistingPath(t, preview.Targets[index].Target, want)
		} else if preview.Targets[index].Target != want {
			t.Fatalf("target %d = %q, want %q", index, preview.Targets[index].Target, want)
		}
	}
}
