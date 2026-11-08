package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCheckExplainsQuotaFailureWithoutGuessingBalance(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(claude.ID, "dmx", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	httpClient.status = 403
	httpClient.body = `{"message":"token quota is insufficient; private /private/account-store"}`
	err := cli.Execute(app, []string{"check"})
	if err == nil || !strings.Contains(out.String(), "HTTP 403: the Token's quota is exhausted") || !strings.Contains(out.String(), "Fix: Add quota to the Token in provider dmx's console.") {
		t.Fatalf("output=%s error=%v", out.String(), err)
	}
	if strings.Contains(out.String(), "/private/account-store") {
		t.Fatalf("check exposed provider response content: %s", out.String())
	}
}

func TestCheckJSONSeparatesHardQuotaFromRateLimit(t *testing.T) {
	for _, test := range []struct {
		name      string
		body      string
		kind      string
		retryable bool
	}{
		{"hard quota", `{"error":{"code":"insufficient_quota"}}`, "quota_exhausted", false},
		{"rate limit", `{"error":{"code":"rate_limit_exceeded"}}`, "rate_limited", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, _, httpClient := testApp(t, "")
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, claude.ID, "claude-test")
			cfg.SetSelectedModel(claude.ID, "dmx", "")
			cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("dmx", "secret"); err != nil {
				t.Fatal(err)
			}
			httpClient.status = http.StatusTooManyRequests
			httpClient.body = test.body
			if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
				t.Fatal("check accepted a failed diagnostic")
			}
			result := decodeCheck(t, out)
			if strings.Contains(out.String(), `"config_path"`) || strings.Contains(out.String(), app.Config.Path()) {
				t.Fatalf("check exposed a private configuration path: %s", out.String())
			}
			client, _ := result.client(claude.ID)
			if result.OK || client.Failure == nil || client.Failure.Kind != test.kind || client.Failure.Retryable != test.retryable || client.NextAction+client.NextDetail == "" {
				t.Fatalf("check JSON = %#v", result)
			}
			if strings.Contains(out.String(), "secret") {
				t.Fatal("check JSON disclosed the Account Token")
			}
		})
	}
}

func TestCheckFailsWhenEnabledClaudeAdapterExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Models["claude-fable-5"] = qualifiedModel("Claude Fable", "dmx", "claude-fable-5", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, "claude-fable-5", "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude-real", nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"check"})
	if err == nil || !strings.Contains(out.String(), "Claude executable is unavailable") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("check did not block on an unavailable Claude executable; err=%v output=%s", err, out.String())
	}
}

func TestCheckJSONReportsOnlyActiveModels(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "claude", "claude-account", "Claude", domain.Endpoints{Anthropic: "https://claude.test"}, claude.ID, "claude-test")
	addProviderModel(&cfg, "unused", "unused-account", "Unused", domain.Endpoints{Anthropic: "https://unused.test"}, claude.ID, "unused-test")
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "claude-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"check", "--json"}); err != nil {
		t.Fatalf("check --json failed: %v\n%s", err, out.String())
	}
	result := decodeCheck(t, out)
	client, ok := result.client(claude.ID)
	if !ok || client.Depth != "model" || client.Model != "claude-account/claude-test" || client.Provider != "claude-account" || !result.OK || len(result.Clients) != 1 {
		t.Fatalf("JSON readiness = %#v", result)
	}
	if strings.Contains(out.String(), "unused") {
		t.Fatalf("JSON readiness exposed an unused model: %s", out)
	}
	if strings.Contains(out.String(), "claude-token") || strings.Contains(out.String(), `"routes"`) {
		t.Fatal("check --json exposed credential material")
	}
}

func TestCheckJSONMakesMissingActiveCredentialActionable(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "claude", "claude-account", "Claude", domain.Endpoints{Anthropic: "https://claude.test"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
		t.Fatal("check --json accepted an active route without its account token")
	}
	result := decodeCheck(t, out)
	client, _ := result.client(claude.ID)
	// Rotation asks a person for the Token, so it is a step for that person.
	if result.OK || client.Failure == nil || client.Failure.Depth != "local" || client.Provider != "claude-account" || client.Failure.Problem != "Provider claude-account has no Token" ||
		client.NextAction != "" || client.NextDetail != "aigw provider rotate claude-account" {
		t.Fatalf("JSON missing-token result = %#v", result)
	}
}

func TestCheckJSONReportsPrerequisiteFailure(t *testing.T) {
	for _, test := range []struct {
		name, version, problem, nextDetail string
	}{
		{"local build without configuration", "0.1.0-dev", "AIGW is not set up", "aigw setup"},
		{"unconfigured", "1.0.0", "AIGW is not set up", "aigw setup"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			app.Version = test.version
			if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
				t.Fatal("check --json accepted an unmet prerequisite")
			}
			var result struct {
				OK         bool   `json:"ok"`
				Problem    string `json:"problem"`
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("decode check --json: %v\n%s", err, out.String())
			}
			// Setup asks a person for an Account and a Route.
			if result.OK || result.Problem != test.problem || result.NextAction != "" || result.NextDetail != test.nextDetail {
				t.Fatalf("prerequisite JSON result = %#v", result)
			}
		})
	}
}

func TestCheckJSONKeepsConfigurationFailureMachineReadable(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Version = "1.0.0"
	if err := os.WriteFile(app.Config.Path(), []byte("version = ["), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
		t.Fatal("check --json accepted malformed configuration")
	}
	// A configuration that cannot load fails with the failure document.
	var result struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		Stage      string `json:"stage"`
		NextAction string `json:"next_action"`
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode malformed-configuration check --json: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), `"config_path"`) || strings.Contains(out.String(), app.Config.Path()) {
		t.Fatalf("check exposed a private configuration path: %s", out.String())
	}
	if result.OK || result.Error == "" || result.Stage != "configuration_parse" || result.NextAction != "" || !strings.Contains(result.NextDetail, "aigw config path") {
		t.Fatalf("malformed-configuration JSON result = %#v", result)
	}
}

func TestCheckSurfacesMissingSelectedModelToken(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err = cli.Execute(app, []string{"check"})
	if err == nil || !strings.Contains(out.String(), "has no Token") || !strings.Contains(out.String(), "Fix: aigw provider rotate ") {
		t.Fatalf("error = %v\n%s", err, out)
	}
	if strings.Contains(out.String(), "gateway") {
		t.Fatalf("missing-token guidance names a gateway: %s", out)
	}
}

func TestCheckProvidesOneClearHealthSummary(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "claude", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"✓ Claude", "dmx/claude-test · model", "No checked client failed"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCheckRejectsAnEnabledClientModelWithoutItsProviderToken(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "claude", "claude-account", "Claude", domain.Endpoints{Anthropic: "https://claude.test"}, claude.ID, "claude-test")
	addProviderModel(&cfg, "codex", "codex-account", "Codex", domain.Endpoints{OpenAIResponses: "https://codex.test/v1"}, codex.ID, "gpt-test")
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetSelectedModel(codex.ID, "codex", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, nil, 0o600)
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	clientRuntime, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	clientRuntime.CredentialCommand = app.Executable
	if _, err := codex.ReconcileConfigs(nil, []codex.TargetRef{codex.HomeTarget(target)}, clientRuntime, clientRuntime, false); err != nil {
		t.Fatal(err)
	}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "claude-token"); err != nil {
		t.Fatal(err)
	}

	err = cli.Execute(app, []string{"check"})
	if err == nil {
		t.Fatal("check accepted an enabled Codex route without its account token")
	}
	// The other client still climbs.
	for _, want := range []string{"✗ Codex", "Provider codex-account has no Token", "Fix: aigw provider rotate codex-account", "✓ Claude", "claude-account/claude-test · model"} {
		if !strings.Contains(out.String(), want) && !strings.Contains(err.Error(), want) {
			t.Fatalf("check output lacks %q:\n%s\nerror: %v", want, out.String(), err)
		}
	}
	if strings.Contains(out.String(), "Everything is healthy") {
		t.Fatalf("check claimed health without every enabled route token:\n%s", out.String())
	}
}

func TestCheckProbesEveryEnabledClientModelAndIgnoresUnselectedModel(t *testing.T) {
	app, out, secretStore, runner, httpClient := testApp(t, "")
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	writeFile(t, codexTarget, []byte("model_provider = \"native\"\n"), 0o600)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers = map[string]domain.Provider{
		"stale-account":  {Label: "Stale", Endpoints: domain.Endpoints{OpenAIResponses: "https://stale.test/v1"}, BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}},
		"claude-account": {Label: "Claude", Endpoints: domain.Endpoints{Anthropic: "https://claude.test"}, BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}},
		"codex-account":  {Label: "Codex", Endpoints: domain.Endpoints{OpenAIResponses: "https://codex.test/v1"}, BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}},
	}
	cfg.Models = map[string]domain.Model{
		"stale":  qualifiedModel("Stale", "stale-account", "stale-model", domain.ProtocolOpenAIResponses),
		"claude": qualifiedModel("Claude", "claude-account", "claude-test", domain.ProtocolAnthropic),
		"codex":  qualifiedModel("Codex", "codex-account", "gpt-test", domain.ProtocolOpenAIResponses),
	}
	cfg.Clients = map[string]domain.ClientBinding{
		claude.ID: {Model: "claude", Enabled: true, Executable: executableFixture(t, "claude")},
		codex.ID:  {Model: "codex", Enabled: true, Executable: "/opt/codex", Targets: []string{codexTarget}},
	}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	codexRuntime, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	codexRuntime.CredentialCommand = app.Executable
	if _, err := codex.ReconcileConfigs(nil, []codex.TargetRef{codex.HomeTarget(codexTarget)}, codexRuntime, codexRuntime, false); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(clients.Admission().IDs()...)); err != nil {
		t.Fatal(err)
	}
	for account, token := range map[string]string{"claude-account": "claude-token", "codex-account": "codex-token", "stale-account": "stale-token"} {
		if err := secretStore.Set(account, token); err != nil {
			t.Fatal(err)
		}
		if err := app.Providers.Set(account, secret.DiagnosticCredential{SystemToken: "system", UserID: "user"}); err != nil {
			t.Fatal(err)
		}
	}
	tokenObservation := &recordingCredentialStore[string]{backend: secretStore}
	diagnosticObservation := &recordingCredentialStore[secret.DiagnosticCredential]{backend: app.Providers}
	app.Secrets = tokenObservation
	app.Providers = diagnosticObservation
	prompt := &scriptedPrompt{}
	app.Interactive = true
	app.Prompt = prompt
	before := map[string][]byte{
		app.Config.Path():                                    readFile(t, app.Config.Path()),
		app.Config.Path() + ".verified.json":                 readFile(t, app.Config.Path()+".verified.json"),
		codexTarget:                                          readFile(t, codexTarget),
		codexTarget + ".aigw-state.json":                     readFile(t, codexTarget+".aigw-state.json"),
		claudeSettingsPath(t, app.Host):                      readFile(t, claudeSettingsPath(t, app.Host)),
		claudeSettingsPath(t, app.Host) + ".aigw-state.json": readFile(t, claudeSettingsPath(t, app.Host)+".aigw-state.json"),
	}
	beforeDirectories := map[string][]string{
		filepath.Dir(app.Config.Path()):               directoryNames(t, filepath.Dir(app.Config.Path())),
		filepath.Dir(codexTarget):                     directoryNames(t, filepath.Dir(codexTarget)),
		filepath.Dir(claudeSettingsPath(t, app.Host)): directoryNames(t, filepath.Dir(claudeSettingsPath(t, app.Host))),
	}
	seen := map[string]int{}
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		seen[req.URL.Host]++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(fixtureResponseBody(req))), Request: req}, nil
	}

	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(seen, map[string]int{"claude.test": 2, "codex.test": 2}) {
		t.Fatalf("probed endpoints = %#v", seen)
	}
	if !slices.Equal(tokenObservation.getCalls, []string{"claude-account", "codex-account"}) {
		t.Fatalf("Token reads = %q, want only enabled Route Accounts", tokenObservation.getCalls)
	}
	for operation, calls := range map[string][]string{
		"diagnostic read": diagnosticObservation.getCalls, "diagnostic metadata": diagnosticObservation.existsCalls,
		"Token set": tokenObservation.setCalls, "Token delete": tokenObservation.deleteCalls,
		"diagnostic set": diagnosticObservation.setCalls, "diagnostic delete": diagnosticObservation.deleteCalls,
	} {
		if len(calls) != 0 {
			t.Fatalf("check performed %s: %q", operation, calls)
		}
	}
	if prompt.callCount() != 0 {
		t.Fatalf("check prompted %d times", prompt.callCount())
	}
	if len(runner.plans) != 0 {
		t.Fatalf("check started a client process: %#v", runner.plans)
	}
	for path, want := range before {
		if after := readFile(t, path); !bytes.Equal(after, want) {
			t.Fatalf("check changed %s", path)
		}
	}
	for directory, want := range beforeDirectories {
		if after := directoryNames(t, directory); !slices.Equal(after, want) {
			t.Fatalf("check changed directory %s: before=%q after=%q", directory, want, after)
		}
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Codex") || strings.Contains(out.String(), "Stale") {
		t.Fatalf("check output does not describe active Client Bindings:\n%s", out.String())
	}
}

func TestCheckEvaluatesModelsIndependentlyOfProgramVersion(t *testing.T) {
	for _, version := range []string{"", "0.1.0-rc.44+local.test", "0.1.0-dev", "1.2.3"} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(version+map[bool]string{false: "/human", true: "/json"}[jsonMode], func(t *testing.T) {
				app, out, store, _, _ := testApp(t, "")
				app.Version = version
				cfg := domain.NewConfig(clients.Admission())
				addProviderModel(&cfg, "claude", "account", "Claude", domain.Endpoints{Anthropic: "https://claude.test"}, claude.ID, "claude-test")
				cfg.SetSelectedModel(claude.ID, "claude", "")
				cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
				synchronizeClaudeProjection(t, app, cfg)
				if err := app.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				args := []string{"check"}
				if jsonMode {
					args = append(args, "--json")
				}
				if err := cli.Execute(app, args); err == nil || !strings.Contains(out.String(), "Provider account has no Token") {
					t.Fatalf("missing credential must remain actionable: error=%v output=%s", err, out.String())
				}
				if err := store.Set("account", "synthetic-token"); err != nil {
					t.Fatal(err)
				}
				out.Reset()
				if err := cli.Execute(app, args); err != nil {
					t.Fatalf("configured route rejected for version %q: %v; %s", version, err, out.String())
				}
			})
		}
	}
}

func TestCheckKeepsGenericHealthAvailableWhenExactDiagnosticDriverIsNotBundled(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["future"] = domain.Provider{
		Label:        "Future Gateway",
		Endpoints:    domain.Endpoints{Anthropic: "https://future.test"},
		BalanceProbe: &domain.BalanceProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Models["claude"] = qualifiedModel("Claude", "future", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("future", "test-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "future/claude-test · model") || strings.Contains(out.String(), "balance") {
		t.Fatalf("check output = %s", out.String())
	}
}
