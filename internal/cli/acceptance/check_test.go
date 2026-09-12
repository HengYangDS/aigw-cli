package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/codex"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"bytes"
	"encoding/json"
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
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "dmx"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	httpClient.status = 403
	httpClient.body = `{"message":"token quota is insufficient"}`
	err := cli.Execute(app, []string{"check"})
	if err == nil || !strings.Contains(out.String()+err.Error(), "Token quota is exhausted") || !strings.Contains(out.String()+err.Error(), "Increase the Token quota for Account dmx in the provider console") {
		t.Fatalf("output=%s error=%v", out.String(), err)
	}
}

func TestCheckFailsWhenEnabledClaudeAdapterExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude Fable", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-fable-5"}
	cfg.Routes[configuration.ClientClaude] = "claude-fable-5"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
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

func TestCheckJSONReportsOnlyActiveRoutes(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude-account", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	addAccountProfile(&cfg, "unused", "unused-account", "Unused", configuration.Endpoints{Anthropic: "https://unused.test"}, configuration.ClientClaude, "unused-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
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
	var result struct {
		Routes map[string]struct {
			Profile     string `json:"profile"`
			Account     string `json:"account"`
			CheckPassed bool   `json:"check_passed"`
		} `json:"routes"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode check --json: %v\n%s", err, out.String())
	}
	route, ok := result.Routes[configuration.ClientClaude]
	if !ok || !route.CheckPassed || route.Profile != "claude" || route.Account != "claude-account" || !result.OK {
		t.Fatalf("JSON readiness = %#v", result)
	}
	if _, present := result.Routes["unused"]; present {
		t.Fatalf("JSON readiness exposed an inactive profile: %#v", result.Routes)
	}
	if strings.Contains(out.String(), "claude-token") {
		t.Fatal("check --json exposed credential material")
	}
}

func TestCheckJSONMakesMissingActiveCredentialActionable(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude-account", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
		t.Fatal("check --json accepted an active route without its account token")
	}
	var result struct {
		Routes map[string]struct {
			Account     string `json:"account"`
			Issue       string `json:"issue"`
			NextAction  string `json:"next_action"`
			CheckPassed bool   `json:"check_passed"`
		} `json:"routes"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode check --json: %v\n%s", err, out.String())
	}
	route := result.Routes[configuration.ClientClaude]
	if result.OK || route.CheckPassed || route.Account != "claude-account" || route.Issue != "account token is unavailable" || route.NextAction != "aigw rotate claude-account" {
		t.Fatalf("JSON missing-token result = %#v", result)
	}
}

func TestCheckJSONReportsPrerequisiteFailure(t *testing.T) {
	for _, test := range []struct {
		name, version, problem, nextAction string
	}{
		{"local build", "0.1.0-dev", "local program is not an official release", "aigw update"},
		{"unconfigured", "1.0.0", "not configured", "aigw setup"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			app.Version = test.version
			if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
				t.Fatal("check --json accepted an unmet prerequisite")
			}
			var result struct {
				OK         bool   `json:"ok"`
				Error      string `json:"error"`
				NextAction string `json:"next_action"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("decode check --json: %v\n%s", err, out.String())
			}
			if result.OK || result.Error != test.problem || result.NextAction != test.nextAction {
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
	var result struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		NextAction string `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode malformed-configuration check --json: %v\n%s", err, out.String())
	}
	if result.OK || result.Error == "" || result.NextAction != "aigw doctor" {
		t.Fatalf("malformed-configuration JSON result = %#v", result)
	}
}

func TestCheckSurfacesMissingSelectedRouteToken(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err = cli.Execute(app, []string{"check"})
	if err == nil || !strings.Contains(err.Error(), "Claude account token is unavailable") {
		t.Fatalf("error = %v", err)
	}
	if output := out.String() + err.Error(); !strings.Contains(output, "selected service endpoint") || strings.Contains(output, "selected gateway") {
		t.Fatalf("missing-token guidance is endpoint-ambiguous: %s", output)
	}
}

func TestCheckProvidesOneClearHealthSummary(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Configuration file", "Claude", "Endpoint checked", "All enabled route checks passed", "Model inference and real-client execution were not verified"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCheckRejectsAnEnabledClientRouteWithoutItsAccountToken(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude-account", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	addAccountProfile(&cfg, "codex", "codex-account", "Codex", configuration.Endpoints{OpenAIResponses: "https://codex.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "claude-token"); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"check"})
	if err == nil {
		t.Fatal("check accepted an enabled Codex route without its account token")
	}
	for _, want := range []string{"Codex account token is unavailable", "codex-account", "aigw rotate codex-account"} {
		if !strings.Contains(out.String(), want) && !strings.Contains(err.Error(), want) {
			t.Fatalf("check output lacks %q:\n%s\nerror: %v", want, out.String(), err)
		}
	}
	if strings.Contains(out.String(), "Everything is healthy") {
		t.Fatalf("check claimed health without every enabled route token:\n%s", out.String())
	}
}

func TestCheckProbesEveryEnabledClientRouteAndIgnoresUnselectedProfile(t *testing.T) {
	app, out, secretStore, runner, httpClient := testApp(t, "")
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	writeFile(t, codexTarget, []byte("model_provider = \"native\"\n"), 0o600)
	cfg := configuration.Config{
		Version: configuration.ConfigVersion,
		Accounts: map[string]configuration.Account{
			"stale-account":  {Label: "Stale", Endpoints: configuration.Endpoints{OpenAIResponses: "https://stale.test/v1"}, AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}},
			"claude-account": {Label: "Claude", Endpoints: configuration.Endpoints{Anthropic: "https://claude.test"}, AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}},
			"codex-account":  {Label: "Codex", Endpoints: configuration.Endpoints{OpenAIResponses: "https://codex.test/v1"}, AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}},
		},
		Profiles: map[string]configuration.Profile{
			"stale":  {Label: "Stale", Account: "stale-account", Client: configuration.ClientCodex, Model: "stale-model"},
			"claude": {Label: "Claude", Account: "claude-account", Client: configuration.ClientClaude, Model: "claude-test"},
			"codex":  {Label: "Codex", Account: "codex-account", Client: configuration.ClientCodex, Model: "gpt-test"},
		},
		Routes: configuration.Routes{configuration.ClientClaude: "claude", configuration.ClientCodex: "codex"},
		Adapters: map[string]configuration.AdapterConfig{
			configuration.ClientClaude: {Enabled: true, Executable: executableFixture(t, "claude")},
			configuration.ClientCodex:  {Enabled: true, Executable: "/opt/codex", Targets: []string{codexTarget}},
		},
	}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	codexRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	codexRuntime.CredentialCommand = app.Executable
	if err := codex.SyncConfig(codexTarget, codexRuntime); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), cfg, configuration.AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}
	for account, token := range map[string]string{"claude-account": "claude-token", "codex-account": "codex-token", "stale-account": "stale-token"} {
		if err := secretStore.Set(account, token); err != nil {
			t.Fatal(err)
		}
		if err := app.Accounts.Set(account, secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"}); err != nil {
			t.Fatal(err)
		}
	}
	tokenObservation := &recordingCredentialStore[string]{backend: secretStore}
	diagnosticObservation := &recordingCredentialStore[secrets.DiagnosticCredential]{backend: app.Accounts}
	app.Secrets = tokenObservation
	app.Accounts = diagnosticObservation
	prompt := &scriptedPrompt{}
	app.Interactive = true
	app.Prompt = prompt
	before := map[string][]byte{
		app.Config.Path():                           readFile(t, app.Config.Path()),
		app.Config.Path() + ".verified.json":        readFile(t, app.Config.Path()+".verified.json"),
		codexTarget:                                 readFile(t, codexTarget),
		codexTarget + ".aigw-state.json":            readFile(t, codexTarget+".aigw-state.json"),
		app.ClaudeSettingsPath:                      readFile(t, app.ClaudeSettingsPath),
		app.ClaudeSettingsPath + ".aigw-state.json": readFile(t, app.ClaudeSettingsPath+".aigw-state.json"),
	}
	beforeDirectories := map[string][]string{
		filepath.Dir(app.Config.Path()):      directoryNames(t, filepath.Dir(app.Config.Path())),
		filepath.Dir(codexTarget):            directoryNames(t, filepath.Dir(codexTarget)),
		filepath.Dir(app.ClaudeSettingsPath): directoryNames(t, filepath.Dir(app.ClaudeSettingsPath)),
	}
	seen := map[string]int{}
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		seen[req.URL.Host]++
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
	}

	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(seen, map[string]int{"claude.test": 1, "codex.test": 1}) {
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
		t.Fatalf("check output does not describe active client Routes:\n%s", out.String())
	}
}

func TestCheckDoesNotDescribeRemoteHTTPSAsExternalLoopbackTransport(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "remote", "remote", "Remote Gateway", configuration.Endpoints{Anthropic: "https://gateway.test"}, configuration.ClientClaude, "model-test")
	cfg.Routes[configuration.ClientClaude] = "remote"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("remote", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "External loopback compatibility layer") {
		t.Fatalf("check misclassified remote endpoint:\n%s", out.String())
	}
}

func TestCheckRejectsLocalProgramBuildBeforeClaimingHealth(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Version = "0.1.0-rc.44+local.test"
	err := cli.Execute(app, []string{"check"})
	if err == nil {
		t.Fatal("check succeeded for a local program build")
	}
	for _, want := range []string{"Local program is not an official release", "Detected local build marker", "aigw update"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check output missing %q:\n%s", want, out.String())
		}
	}
}

func TestCheckRejectsDefaultDevelopmentProgramBuildBeforeClaimingHealth(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Version = "0.1.0-dev"
	err := cli.Execute(app, []string{"check"})
	if err == nil {
		t.Fatal("check succeeded for the default development program build")
	}
	for _, want := range []string{"Local program is not an official release", "Detected local build marker", "aigw update"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check output missing %q:\n%s", want, out.String())
		}
	}
}

func TestCheckKeepsGenericHealthAvailableWhenExactDiagnosticDriverIsNotBundled(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["future"] = configuration.Account{
		Label:        "Future Gateway",
		Endpoints:    configuration.Endpoints{Anthropic: "https://future.test"},
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "future", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
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
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Endpoint checked") || strings.Contains(out.String(), "aigw balance") {
		t.Fatalf("check output = %s", out.String())
	}
}
