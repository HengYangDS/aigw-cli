package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUseSurfacesCredentialObservationFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), existsErr: want}

	if err := cli.Execute(app, []string{"use", "claude", "one/claude-test"}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestUseSelectsClientNativeModelWithoutAccessingProviderTokens(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	credentials := &recordingCredentialStore[string]{backend: secret.NewMemoryStore()}
	app.Secrets = credentials
	target := filepath.Join(t.TempDir(), "config.toml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{codex.ID: executableFixture(t, "codex")},
		Surfaces: []discovery.Surface{{
			ID:          string(codex.HomeDefault),
			ConfigPath:  target,
			AutoManaged: true,
		}},
	}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["aws"] = domain.Provider{
		Label:     "AWS Bedrock",
		Endpoints: domain.Endpoints{Responses: "https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1"},
	}
	cfg.Models[domain.NewModelRef("aws", "openai.gpt-5.6-sol")] = domain.Model{
		Label:    "AWS Bedrock",
		Provider: "aws",
		Family:   "openai.gpt-5.6-sol", ID: "openai.gpt-5.6-sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolResponses: {},
		},
	}
	cfg.Clients[codex.ID] = domain.ClientBinding{
		Model: domain.NewModelRef("aws", "openai.gpt-5.6-sol"), ModelProvider: "amazon-bedrock",
		Authentication: domain.AuthenticationClientNative,
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"use", "codex", "aws/openai.gpt-5.6-sol"}); err != nil {
		t.Fatalf("select client-native route: %v", err)
	}
	if len(credentials.getCalls)+len(credentials.existsCalls)+len(credentials.setCalls)+len(credentials.deleteCalls) != 0 {
		t.Fatal("client-native selection accessed AIGW credentials")
	}
	selected, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := selected.SelectedModel(codex.ID); got != domain.NewModelRef("aws", "openai.gpt-5.6-sol") {
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
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	cfg.Models[domain.NewModelRef("gateway", "gpt-test")] = qualifiedModel("GPT", "gateway", "gpt-test", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("gateway", "claude-fable")] = qualifiedModel("Claude Fable", "gateway", "claude-fable", domain.ProtocolAnthropic)
	cfg.Models[domain.NewModelRef("gateway", "claude-sonnet")] = qualifiedModel("Claude Sonnet", "gateway", "claude-sonnet", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-test"), "")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-fable"), "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
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

	if err := cli.Execute(app, []string{"use", "claude", "gateway/claude-sonnet"}); err != nil {
		t.Fatalf("Claude-only route change touched Codex target: %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-test") || got.SelectedModel(claude.ID) != domain.NewModelRef("gateway", "claude-sonnet") {
		t.Fatalf("client bindings = %#v", got.Clients)
	}
	if after := readFile(t, target); !bytes.Equal(after, codexProjection) {
		t.Fatal("Claude selection rewrote the independent Codex projection")
	}
	if after := readFile(t, target+".aigw-state.json"); !bytes.Equal(after, codexState) {
		t.Fatal("Claude selection rewrote the independent Codex projection state")
	}
	if projected := readFile(t, claudeSettingsPath(t, app.Host)); !bytes.Contains(projected, []byte("claude-sonnet")) {
		t.Fatal("Claude selection did not update its own projection")
	}
}

func TestUseForCodexLeavesUnselectedClaudeDriftUntouched(t *testing.T) {
	app, _, credentials, _, _ := testApp(t, "")
	cfg := twoModelConfig()
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	cfg.Models[domain.NewModelRef("one", "claude-test")] = qualifiedModel("Claude", "one", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-test"), "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
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
	codexProjection := string(readFile(t, target))
	codexProjection = strings.Replace(codexProjection, `model_provider = "aigw" # managed by AIGW`, `model_provider = "native"`, 1)
	codexProjection = strings.Replace(codexProjection, `model = "model-one" # managed by AIGW`, `model = "model-user"`, 1)
	if err := os.WriteFile(target, []byte(codexProjection), 0o600); err != nil {
		t.Fatal(err)
	}
	state := readFile(t, claudeSettingsPath(t, app.Host)+".aigw-state.json")
	foreign := []byte(`{"apiKeyHelper":"user-owned-helper"}`)
	if err := os.WriteFile(claudeSettingsPath(t, app.Host), foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"use", "codex", "two/model-two"}); err != nil {
		t.Fatalf("Codex selection was blocked by an unselected Claude projection: %v", err)
	}
	after, err := app.Config.Load()
	if err != nil || after.SelectedModel(codex.ID) != domain.NewModelRef("two", "model-two") || after.SelectedModel(claude.ID) != domain.NewModelRef("one", "claude-test") {
		t.Fatalf("client-scoped selection = %#v, %v", after.Clients, err)
	}
	if !bytes.Equal(readFile(t, claudeSettingsPath(t, app.Host)), foreign) || !bytes.Equal(readFile(t, claudeSettingsPath(t, app.Host)+".aigw-state.json"), state) {
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
			claude.ID: claudeExecutable,
			codex.ID:  codexExecutable,
		},
		Surfaces: []discovery.Surface{{
			ID:          string(codex.HomeDefault),
			ConfigPath:  codexTarget,
			AutoManaged: true,
		}},
	}}
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "claude-gateway", "Claude", domain.Endpoints{Anthropic: "https://claude.test"}, claude.ID, "claude-test")
	addProviderModel(&cfg, "codex-gateway", "Codex", domain.Endpoints{Responses: "https://codex.test/v1"}, codex.ID, "gpt-test")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for account, token := range map[string]string{"claude-gateway": "claude-token", "codex-gateway": "codex-token"} {
		if err := secretStore.Set(account, token); err != nil {
			t.Fatal(err)
		}
	}

	if err := cli.Execute(app, []string{"use", "claude", "claude-gateway/claude-test"}); err != nil {
		t.Fatalf("select Claude route: %v", err)
	}
	claudeProjection, err := os.ReadFile(claudeSettingsPath(t, app.Host))
	if err != nil {
		t.Fatalf("read Claude projection: %v", err)
	}
	claudeState, err := os.ReadFile(claudeSettingsPath(t, app.Host) + ".aigw-state.json")
	if err != nil {
		t.Fatalf("read Claude projection state: %v", err)
	}

	if err := cli.Execute(app, []string{"use", "codex", "codex-gateway/gpt-test"}); err != nil {
		t.Fatalf("select Codex route: %v", err)
	}
	claudeAfterCodex, err := os.ReadFile(claudeSettingsPath(t, app.Host))
	if err != nil {
		t.Fatalf("read Claude projection after Codex selection: %v", err)
	}
	if !bytes.Equal(claudeAfterCodex, claudeProjection) {
		t.Fatal("Codex selection rewrote the independent Claude projection")
	}
	if after := readFile(t, claudeSettingsPath(t, app.Host)+".aigw-state.json"); !bytes.Equal(after, claudeState) {
		t.Fatal("Codex selection rewrote the independent Claude projection state")
	}
	selected, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(claude.ID) != domain.NewModelRef("claude-gateway", "claude-test") || selected.SelectedModel(codex.ID) != domain.NewModelRef("codex-gateway", "gpt-test") {
		t.Fatalf("independent client bindings = %#v", selected.Clients)
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
		t.Fatalf("check did not accept both independent Client Bindings:\n%s", out.String())
	}
}

func TestRepeatedUseOfActiveModelDoesNotRewriteOwnedState(t *testing.T) {
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
		claude.ID: claudeExecutable,
	}}}
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "gateway", "Claude", domain.Endpoints{Anthropic: "https://claude.test"}, claude.ID, "claude-test")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"use", "claude", "gateway/claude-test"}); err != nil {
		t.Fatalf("initial selection: %v", err)
	}
	selected, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), selected, sessions(claude.ID)); err != nil {
		t.Fatal(err)
	}

	ownedPaths := []string{
		app.Config.Path(),
		app.Config.Path() + ".bak",
		verifiedPath(app.Config.Path()),
		claudeSettingsPath(t, app.Host),
		claudeSettingsPath(t, app.Host) + ".aigw-state.json",
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

	if err := cli.Execute(app, []string{"use", "claude", "gateway/claude-test"}); err != nil {
		t.Fatalf("repeat active selection: %v", err)
	}
	if text := out.String(); !strings.Contains(text, "Model already selected") || strings.Contains(text, "Model selected") {
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
		claude.ID: claudeExecutable,
	}}}
	if err := secretStore.Set("dmxapi", "test-token"); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := cli.Execute(app, []string{"use", "claude", "dmxapi/claude-test"}); err != nil {
		t.Fatalf("use after installing Claude: %v", err)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := after.Clients[claude.ID]
	if !adapter.Enabled || adapter.Executable != claudeExecutable {
		t.Fatalf("Claude adapter after use = %#v", adapter)
	}
	settings, err := os.ReadFile(claudeSettingsPath(t, app.Host))
	if err != nil {
		t.Fatalf("read Claude projection: %v", err)
	}
	if !strings.Contains(string(settings), "https://dmxapi.test") || !strings.Contains(string(settings), "claude-test") {
		t.Fatalf("use did not project the selected Claude route:\n%s", settings)
	}

	out.Reset()
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatalf("check after use: %v", err)
	}
	if !strings.Contains(out.String(), "✓ Claude") || !strings.Contains(out.String(), "dmxapi/claude-test · model") || strings.Contains(out.String(), "No client is enabled") {
		t.Fatalf("check did not verify the activated Claude route:\n%s", out.String())
	}
}

func TestUseRollsBackModelWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	dir := t.TempDir()
	target := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, codex.ID, "gpt-one")
	addProviderModel(&cfg, "two", "Two", domain.Endpoints{Responses: "https://two.test/v1"}, codex.ID, "gpt-two")
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("one", "gpt-one"), "")
	cfg.SetClientActivation(codex.ID, true, "/missing/codex", []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-secret")
	_ = secretStore.Set("two", "new-secret")
	app.Executable = "relative-helper"
	// The selection's preview plans Codex's file, so the adapter's refusal
	// comes before anything is written, and nothing claims a rollback.
	err := cli.Execute(app, []string{"use", "codex", "two/gpt-two"})
	if err == nil || !strings.Contains(err.Error(), "credential command must be absolute") || strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("error = %v", err)
	}
	got, _ := app.Config.Load()
	if got.SelectedModel(codex.ID) != domain.NewModelRef("one", "gpt-one") {
		t.Fatalf("selection was not rolled back: %#v", got.Clients)
	}
}

func TestUseCodexModelOnSameProviderDoesNotRebindCredentials(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMX", Endpoints: domain.Endpoints{Responses: "https://example.test/v1"}}
	cfg.Models[domain.NewModelRef("dmx", "gpt-5.6-sol")] = qualifiedModel("Sol", "dmx", "gpt-5.6-sol", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("dmx", "gpt-5.6-terra")] = qualifiedModel("Terra", "dmx", "gpt-5.6-terra", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-5.6-sol"), "")
	cfg.SetClientActivation(codex.ID, true, "/usr/local/bin/codex", []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"use", "codex", "dmx/gpt-5.6-terra"}); err != nil {
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
	app.Config = store.NewStore(t.TempDir(), clients.Admission())
	err := cli.Execute(app, []string{"use", "claude", "one/claude-test"})
	if err == nil {
		t.Fatal("expected a config load failure")
	}
}

func TestUseRejectsUnknownModel(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-one"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	// A model that is not added is added once the provider's catalog lists
	// it; a catalog that cannot be read refuses it before any write, naming
	// --unlisted, which adds an ID that the provider serves unlisted.
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	err = cli.Execute(app, []string{"use", "claude", "one/does-not-exist"})
	if problem := terminal.ProblemFor(err); err == nil || !strings.Contains(err.Error(), "the one catalog cannot be read") ||
		problem.Fix != "aigw use claude one/does-not-exist --unlisted" {
		t.Fatalf("error = %v, fix %q", err, problem.Fix)
	}
	if after, err := app.Config.CaptureSnapshot(); err != nil || !after.Config.Equal(before.Config) {
		t.Fatalf("a refused addition wrote the configuration: %v", err)
	}
}

func TestUseSurfacesUnknownProviderReference(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Models[domain.NewModelRef("one", "claude-one")] = qualifiedModel("One", "one", "claude-one", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-one"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	// Store.Save validates referential integrity, so a selection of a
	// provider that the file does not hold can only come from a file edited
	// outside AIGW, by hand or by another tool, after the fact.
	data, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n[clients.codex]\nenabled = false\nmodel = \"ghost/claude-broken\"\n")...)
	if err := os.WriteFile(app.Config.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	// Loading the configuration refuses the selection, before any Token is
	// read.
	err = cli.Execute(app, []string{"use", "claude", "ghost/claude-broken"})
	if err == nil || !strings.Contains(err.Error(), "names the unknown model ghost/claude-broken") {
		t.Fatalf("error = %v", err)
	}
}
