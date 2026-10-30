package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/client"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/discovery"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"bytes"
	"encoding/json"
	"errors"
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
		cfg := domain.NewConfig(clients.Admission())
		addAccountRoute(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt")
		cfg.SetSelectedRoute(codex.ID, "one", "")
		cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
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
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "team", "team", "Team", domain.Endpoints{OpenAIResponses: "https://team.test/v1"}, codex.ID, "team-model")
	cfg.SetSelectedRoute(codex.ID, "team", "")
	cfg.SetClientActivation("codex", true, "/opt/codex-real", targets)
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
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Routes["gpt"] = qualifiedRoute("GPT", "dmx", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	cfg.SetClientActivation(codex.ID, true, "/usr/local/bin/codex", []string{target})
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

func TestSyncUsesSharedCodexHomeAndOfficialClaudeSettingsWithoutTouchingClientState(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	for _, name := range []string{"aigw", claude.ID, codex.ID} {
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
	app.Host = hostAt(home)
	app.Discovery = client.NewDiscoverer(clients.Registry(), discovery.System{GOOS: runtime.GOOS, Home: home, Path: bin})
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	cfg.Routes = map[string]domain.Route{
		"claude": qualifiedRoute("Claude", "gateway", "claude-team", domain.ProtocolAnthropic),
		"codex":  qualifiedRoute("Codex", "gateway", "gpt-team", domain.ProtocolOpenAIResponses),
	}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	cfg.SetClientActivation(claude.ID, true, "", nil)
	cfg.SetClientActivation(codex.ID, true, "", nil)
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
	requireRouteSelectionsUnchanged(t, cfg, after)
	adapter := after.Clients[codex.ID]
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
	requireFilesUnchanged(t, preserved)
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
	projection, err := after.ResolveRuntime(claude.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(helper, app.Executable) || !strings.HasSuffix(helper, " credential claude "+projection.CredentialProjectionFingerprint(claude.ID)) {
		t.Fatalf("Claude helper is not the absolute AIGW credential command: %q", helper)
	}
	if strings.Contains(string(data), "plaintext-token-must-not-be-projected") {
		t.Fatalf("Claude settings contain credential material: %s", data)
	}
}

func requireFilesUnchanged(t *testing.T, files map[string][]byte) {
	t.Helper()
	for path, want := range files {
		if got := readFile(t, path); !bytes.Equal(got, want) {
			t.Fatalf("client-owned state changed at %s: %q", path, got)
		}
	}
}

func requireRouteSelectionsUnchanged(t *testing.T, before, after domain.Config) {
	t.Helper()
	if !reflect.DeepEqual(after.Routes, before.Routes) ||
		after.SelectedRoute(claude.ID) != before.SelectedRoute(claude.ID) ||
		after.SelectedRoute(codex.ID) != before.SelectedRoute(codex.ID) {
		t.Fatalf("sync changed Route or client-selection authority: routes=%#v clients=%#v", after.Routes, after.Clients)
	}
}

func TestSyncRefreshesTheClaudeHelperAfterAIGWMoves(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	settingsHome := t.TempDir()
	settingsPath := filepath.Join(settingsHome, ".claude", "settings.json")
	app.Host = hostAt(settingsHome)
	claudeExecutable := executableFixture(t, claude.ID)
	oldAIGWExecutable := executableFixture(t, "aigw-old")
	newAIGWExecutable := executableFixture(t, "aigw-new")
	app.Executable = oldAIGWExecutable
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{
		claude.ID: claudeExecutable,
	}}}

	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "claude", "gateway", "Gateway", domain.Endpoints{Anthropic: "https://gateway.test"}, claude.ID, "claude-team")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, "", nil)
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
	if !reflect.DeepEqual(after.Routes, cfg.Routes) ||
		after.SelectedRoute(claude.ID) != cfg.SelectedRoute(claude.ID) {
		t.Fatalf("sync changed Route or client-selection authority: routes=%#v clients=%#v", after.Routes, after.Clients)
	}
	if after.Clients[claude.ID].Executable != claudeExecutable {
		t.Fatalf("sync changed the Claude executable: %#v", after.Clients[claude.ID])
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
	saveCommandRoute(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(codex.ID, true, "", nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), existsErr: want}

	if err := cli.Execute(app, []string{"sync"}); err == nil || err.Error() != "Synchronization prerequisites are unavailable" || !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	for _, expected := range []string{
		"Synchronization prerequisites are unavailable",
		"AIGW could not determine which selected Client Bindings can be projected with the currently available clients and credentials.",
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
	claudeSettings := filepath.Join(dir, ".claude", "settings.json")
	app.Host = hostAt(dir)
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: "/usr/local/bin/claude"}}}
	for _, target := range []string{first, second} {
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "http://127.0.0.1:48721/v1", Anthropic: "https://gateway.test"}}
	cfg.Routes["terra"] = qualifiedRoute("GPT-5.6 Terra", "gateway", "gpt-5.6-terra", domain.ProtocolOpenAIResponses)
	cfg.Routes["claude"] = qualifiedRoute("Claude", "gateway", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedRoute(codex.ID, "terra", "")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(codex.ID, true, "/usr/local/bin/codex", []string{first, second})
	cfg.SetClientActivation(claude.ID, true, "", nil)
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
		DryRun     bool   `json:"dry_run"`
		NextAction string `json:"next_action"`
		Targets    []struct {
			Client string `json:"client"`
			Target string `json:"target"`
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("decode sync dry-run JSON: %v\n%s", err, out.String())
	}
	wantTargets := []string{claudeSettings, first, second}
	wantClients := []string{claude.ID, codex.ID, codex.ID}
	wantActions := []string{"project", "initial-project", "initial-project"}
	if !preview.DryRun || preview.NextAction != "aigw sync" || len(preview.Targets) != len(wantTargets) {
		t.Fatalf("sync dry-run preview = %#v", preview)
	}
	for index, want := range wantTargets {
		if preview.Targets[index].Client != wantClients[index] {
			t.Fatalf("target %d client = %q, want %q", index, preview.Targets[index].Client, wantClients[index])
		}
		if preview.Targets[index].Action != wantActions[index] {
			t.Fatalf("target %d action = %q, want %q", index, preview.Targets[index].Action, wantActions[index])
		}
		if wantClients[index] == codex.ID {
			assertSameExistingPath(t, preview.Targets[index].Target, want)
		} else if preview.Targets[index].Target != want {
			t.Fatalf("target %d = %q, want %q", index, preview.Targets[index].Target, want)
		}
	}
}

func TestSyncDryRunRecommendsCheckWhenProjectionIsConverged(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	app.Host = hostAt(t.TempDir())
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: "/usr/local/bin/claude"}}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["claude"] = qualifiedRoute("Claude", "gateway", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, "", nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "dry-run-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Targets    []client.ProjectionPlan `json:"targets"`
		NextAction string                  `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 1 || result.Targets[0].Action != "already-converged" || result.NextAction != "aigw check" {
		t.Fatalf("converged sync dry-run preview = %#v", result)
	}
}
