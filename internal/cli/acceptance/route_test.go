package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/surface"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUseSurfacesCredentialObservationFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: want}

	if err := cli.Execute(app, []string{"use", "one"}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestUseSelectsClientNativeProfileWithoutAccessingAccountTokens(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	credentials := &recordingCredentialStore[string]{backend: secrets.NewMemoryStore()}
	app.Secrets = credentials
	target := filepath.Join(t.TempDir(), "config.toml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: executableFixture(t, "codex")},
		Surfaces: []discovery.Surface{{
			ID:          string(surface.CodexHomeDefault),
			Authority:   string(surface.AuthorityAIGW),
			ConfigPath:  target,
			AutoManaged: true,
		}},
	}}
	cfg := configuration.NewConfig()
	cfg.Accounts["aws"] = configuration.Account{
		Label:     "AWS Bedrock",
		Endpoints: configuration.Endpoints{OpenAIResponses: "https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1"},
	}
	cfg.Profiles["bedrock"] = configuration.Profile{
		Label:          "AWS Bedrock",
		Account:        "aws",
		Client:         configuration.ClientCodex,
		Model:          "openai.gpt-5.6-sol",
		ModelProvider:  "amazon-bedrock",
		Authentication: configuration.AuthenticationClientNative,
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"use", "bedrock"}); err != nil {
		t.Fatalf("select client-native profile: %v", err)
	}
	if len(credentials.getCalls)+len(credentials.existsCalls)+len(credentials.setCalls)+len(credentials.deleteCalls) != 0 {
		t.Fatal("client-native selection accessed AIGW credentials")
	}
	selected, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := selected.Routes[configuration.ClientCodex]; got != "bedrock" {
		t.Fatalf("Codex route = %q", got)
	}
	projection := string(readFile(t, target))
	for _, want := range []string{`model_provider = "amazon-bedrock"`, `[model_providers.amazon-bedrock]`} {
		if !strings.Contains(projection, want) {
			t.Fatalf("Codex projection lacks %q:\n%s", want, projection)
		}
	}
	for _, forbidden := range []string{"credential", "auth]"} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("Codex projection contains AIGW Token material %q:\n%s", forbidden, projection)
		}
	}
}

func TestUseForClaudeLeavesUnselectedCodexDriftUntouched(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude-fable"] = configuration.Profile{Label: "Claude Fable", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-fable"}
	cfg.Profiles["claude-sonnet"] = configuration.Profile{Label: "Claude Sonnet", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-sonnet"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude-fable"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{target}}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "test-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project initial Codex route: %v", err)
	}
	codexProjection, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read initial Codex projection: %v", err)
	}
	codexProjection = append(codexProjection, []byte("\nexternal-edit = [\n")...)
	if err := os.WriteFile(target, codexProjection, 0o600); err != nil {
		t.Fatal(err)
	}
	codexState, err := os.ReadFile(target + ".aigw-state.json")
	if err != nil {
		t.Fatalf("read initial Codex state: %v", err)
	}

	if err := cli.Execute(app, []string{"use", "claude-sonnet"}); err != nil {
		t.Fatalf("Claude-only route change touched Codex target: %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes[configuration.ClientCodex] != "gpt" || got.Routes[configuration.ClientClaude] != "claude-sonnet" {
		t.Fatalf("routes = %#v", got.Routes)
	}
	if after := readFile(t, target); !bytes.Equal(after, codexProjection) {
		t.Fatal("Claude selection rewrote the independent Codex projection")
	}
	if after := readFile(t, target+".aigw-state.json"); !bytes.Equal(after, codexState) {
		t.Fatal("Claude selection rewrote the independent Codex projection state")
	}
	if projected := readFile(t, app.ClaudeSettingsPath); !bytes.Contains(projected, []byte("claude-sonnet")) {
		t.Fatal("Claude selection did not update its own projection")
	}
}

func TestUseForCodexLeavesUnselectedClaudeDriftUntouched(t *testing.T) {
	app, _, credentials, _, _ := testApp(t, "")
	cfg := twoProfileConfig()
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{target}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "one", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	for _, account := range []string{"one", "two"} {
		if err := credentials.Set(account, "token"); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	state := readFile(t, app.ClaudeSettingsPath+".aigw-state.json")
	foreign := []byte(`{"apiKeyHelper":"user-owned-helper"}`)
	if err := os.WriteFile(app.ClaudeSettingsPath, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"use", "two"}); err != nil {
		t.Fatalf("Codex selection was blocked by an unselected Claude projection: %v", err)
	}
	after, err := app.Config.Load()
	if err != nil || after.Routes[configuration.ClientCodex] != "two" || after.Routes[configuration.ClientClaude] != "claude" {
		t.Fatalf("client-scoped selection = %#v, %v", after.Routes, err)
	}
	if !bytes.Equal(readFile(t, app.ClaudeSettingsPath), foreign) || !bytes.Equal(readFile(t, app.ClaudeSettingsPath+".aigw-state.json"), state) {
		t.Fatal("Codex selection changed unselected Claude files")
	}
	if projected := readFile(t, target); !bytes.Contains(projected, []byte("model-two")) {
		t.Fatal("Codex selection did not update its own projection")
	}
}

func TestIndependentUseCommandsMakeBothClientsReadyWithoutBulkSelection(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	codexExecutable := executableFixture(t, "codex")
	codexTarget := filepath.Join(t.TempDir(), "config.toml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{
			configuration.ClientClaude: claudeExecutable,
			configuration.ClientCodex:  codexExecutable,
		},
		Surfaces: []discovery.Surface{{
			ID:          string(surface.CodexHomeDefault),
			Authority:   string(surface.AuthorityAIGW),
			ConfigPath:  codexTarget,
			AutoManaged: true,
		}},
	}}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude-gateway", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	addAccountProfile(&cfg, "codex", "codex-gateway", "Codex", configuration.Endpoints{OpenAIResponses: "https://codex.test/v1"}, configuration.ClientCodex, "gpt-test")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for account, token := range map[string]string{"claude-gateway": "claude-token", "codex-gateway": "codex-token"} {
		if err := secretStore.Set(account, token); err != nil {
			t.Fatal(err)
		}
	}

	if err := cli.Execute(app, []string{"use", "claude"}); err != nil {
		t.Fatalf("select Claude route: %v", err)
	}
	claudeProjection, err := os.ReadFile(app.ClaudeSettingsPath)
	if err != nil {
		t.Fatalf("read Claude projection: %v", err)
	}
	claudeState, err := os.ReadFile(app.ClaudeSettingsPath + ".aigw-state.json")
	if err != nil {
		t.Fatalf("read Claude projection state: %v", err)
	}

	if err := cli.Execute(app, []string{"use", "codex"}); err != nil {
		t.Fatalf("select Codex route: %v", err)
	}
	claudeAfterCodex, err := os.ReadFile(app.ClaudeSettingsPath)
	if err != nil {
		t.Fatalf("read Claude projection after Codex selection: %v", err)
	}
	if !bytes.Equal(claudeAfterCodex, claudeProjection) {
		t.Fatal("Codex selection rewrote the independent Claude projection")
	}
	if after := readFile(t, app.ClaudeSettingsPath+".aigw-state.json"); !bytes.Equal(after, claudeState) {
		t.Fatal("Codex selection rewrote the independent Claude projection state")
	}
	selected, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if selected.Routes[configuration.ClientClaude] != "claude" || selected.Routes[configuration.ClientCodex] != "codex" {
		t.Fatalf("independent routes = %#v", selected.Routes)
	}
	for account, want := range map[string]string{"claude-gateway": "claude-token", "codex-gateway": "codex-token"} {
		if got, err := secretStore.Get(account); err != nil || got != want {
			t.Fatalf("credential %s = %q, %v; want unchanged", account, got, err)
		}
	}

	out.Reset()
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatalf("check after independent selections: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Codex") {
		t.Fatalf("check did not accept both independently selected Routes:\n%s", out.String())
	}
}

func TestRepeatedUseOfActiveProfileDoesNotRewriteOwnedState(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	if err := secretStore.Set("gateway", "existing-token"); err != nil {
		t.Fatal(err)
	}
	app.Secrets = &recordingCredentialStore[string]{
		backend:   secretStore,
		setErr:    errors.New("credential rewrite"),
		deleteErr: errors.New("credential deletion"),
	}
	claudeExecutable := executableFixture(t, "claude")
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{
		configuration.ClientClaude: claudeExecutable,
	}}}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "gateway", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"use", "claude"}); err != nil {
		t.Fatalf("initial selection: %v", err)
	}
	selected, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), selected, []string{configuration.ClientClaude}); err != nil {
		t.Fatal(err)
	}

	ownedPaths := []string{
		app.Config.Path(),
		app.Config.Path() + ".bak",
		app.Config.Path() + ".verified.json",
		app.ClaudeSettingsPath,
		app.ClaudeSettingsPath + ".aigw-state.json",
	}
	type fileState struct {
		info os.FileInfo
		data []byte
	}
	before := make(map[string]fileState, len(ownedPaths))
	for _, path := range ownedPaths {
		info, err := os.Stat(path)
		if err == nil {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s before repeated use: %v", path, readErr)
			}
			before[path] = fileState{info: info, data: data}
			continue
		}
		if !os.IsNotExist(err) {
			t.Fatalf("inspect %s before repeated use: %v", path, err)
		}
	}
	plansBefore := len(runner.plans)
	out.Reset()

	if err := cli.Execute(app, []string{"use", "claude"}); err != nil {
		t.Fatalf("repeat active selection: %v", err)
	}
	if text := out.String(); !strings.Contains(text, "Service already selected") || strings.Contains(text, "Service switched") {
		t.Fatalf("repeated use did not report its no-op semantics:\n%s", text)
	}

	for _, path := range ownedPaths {
		beforeState, existed := before[path]
		afterInfo, err := os.Stat(path)
		if !existed {
			if !os.IsNotExist(err) {
				t.Fatalf("repeated use created %s", path)
			}
			continue
		}
		if err != nil {
			t.Fatalf("inspect %s after repeated use: %v", path, err)
		}
		afterData, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s after repeated use: %v", path, readErr)
		}
		if !os.SameFile(beforeState.info, afterInfo) || !bytes.Equal(beforeState.data, afterData) {
			t.Fatalf("repeated use replaced %s", path)
		}
	}
	if len(runner.plans) != plansBefore {
		t.Fatalf("repeated use rebound native authentication: plans %d -> %d", plansBefore, len(runner.plans))
	}
}

func TestUseActivatesClaudeInstalledAfterManifestSetup(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatalf("initial manifest setup: %v", err)
	}

	claudeExecutable := executableFixture(t, "claude")
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{
		configuration.ClientClaude: claudeExecutable,
	}}}
	if err := secretStore.Set("dmxapi", "test-token"); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := cli.Execute(app, []string{"use", "dmxapi-claude"}); err != nil {
		t.Fatalf("use after installing Claude: %v", err)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := after.Adapters[configuration.ClientClaude]
	if !adapter.Enabled || adapter.Executable != claudeExecutable {
		t.Fatalf("Claude adapter after use = %#v", adapter)
	}
	settings, err := os.ReadFile(app.ClaudeSettingsPath)
	if err != nil {
		t.Fatalf("read Claude projection: %v", err)
	}
	if !strings.Contains(string(settings), "https://dmxapi.test") || !strings.Contains(string(settings), "claude-test") {
		t.Fatalf("use did not project the selected Claude profile:\n%s", settings)
	}

	out.Reset()
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatalf("check after use: %v", err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Endpoint checked") || strings.Contains(out.String(), "no clients are enabled") {
		t.Fatalf("check did not verify the activated Claude route:\n%s", out.String())
	}
}

func TestUseRollsBackRouteWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	dir := t.TempDir()
	target := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-one")
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{OpenAIResponses: "https://two.test/v1"}, configuration.ClientCodex, "gpt-two")
	cfg.Routes[configuration.ClientCodex] = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/missing/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-secret")
	_ = secretStore.Set("two", "new-secret")
	app.Executable = "relative-helper"
	err := cli.Execute(app, []string{"use", "two"})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") || strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("error = %v", err)
	}
	got, _ := app.Config.Load()
	if got.Routes[configuration.ClientCodex] != "one" {
		t.Fatalf("route was not rolled back: %#v", got.Routes)
	}
}

func TestUseCodexProfileOnSameAccountDoesNotRebindCredentials(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["sol"] = configuration.Profile{Label: "Sol", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Profiles["terra"] = configuration.Profile{Label: "Terra", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-terra"}
	cfg.Routes[configuration.ClientCodex] = "sol"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"use", "terra"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("same-account model switch rebound credentials: %#v", runner.plans)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model = "gpt-5.6-terra" # managed by AIGW`) {
		t.Fatalf("Codex model was not switched:\n%s", data)
	}
}

func TestUseSurfacesConfigLoadFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Config = configuration.NewStore(t.TempDir())
	err := cli.Execute(app, []string{"use", "one"})
	if err == nil {
		t.Fatal("expected a config load failure")
	}
}

func TestUseRejectsUnknownProfile(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	err := cli.Execute(app, []string{"use", "does-not-exist"})
	if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseSurfacesUnknownAccountReference(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientClaude, Model: "claude-one"}
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	// Store.Save validates referential integrity, so a dangling account
	// reference can only reach accountForInput through a file edited
	// outside AIGW (e.g. by hand or by another tool) after the fact.
	data, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n[profiles.broken]\nlabel = \"Broken\"\naccount = \"ghost\"\nclient = \"claude\"\nmodel = \"claude-broken\"\n")...)
	if err := os.WriteFile(app.Config.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	err = cli.Execute(app, []string{"use", "broken"})
	if err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("error = %v", err)
	}
}
