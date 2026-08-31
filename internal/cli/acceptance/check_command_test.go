package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
)

func TestCheckJSONReportsOnlyActiveRoutes(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude-account", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	addAccountProfile(&cfg, "unused", "unused-account", "Unused", configuration.Endpoints{Anthropic: "https://unused.test"}, configuration.ClientClaude, "unused-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "claude-token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check", "--json"); err != nil {
		t.Fatalf("check --json failed: %v\n%s", err, out.String())
	}
	var result struct {
		Routes map[string]struct {
			Profile string `json:"profile"`
			Account string `json:"account"`
			Ready   bool   `json:"ready"`
		} `json:"routes"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode check --json: %v\n%s", err, out.String())
	}
	route, ok := result.Routes[configuration.ClientClaude]
	if !ok || !route.Ready || route.Profile != "claude" || route.Account != "claude-account" || !result.OK {
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
	app, out, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude-account", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "check", "--json"); err == nil {
		t.Fatal("check --json accepted an active route without its account token")
	}
	var result struct {
		Routes map[string]struct {
			Account string `json:"account"`
			Issue   string `json:"issue"`
			Fix     string `json:"fix"`
			Ready   bool   `json:"ready"`
		} `json:"routes"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode check --json: %v\n%s", err, out.String())
	}
	route := result.Routes[configuration.ClientClaude]
	if result.OK || route.Ready || route.Account != "claude-account" || route.Issue != "account token is unavailable" || route.Fix != "aigw rotate claude-account" {
		t.Fatalf("JSON missing-token result = %#v", result)
	}
}

func TestCheckSurfacesMissingSelectedRouteToken(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err = execute(t, app, "check")
	if err == nil || !strings.Contains(err.Error(), "Claude account token is unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckProvidesOneClearHealthSummary(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Configuration file", "Claude", "Ready", "Every enabled client route is healthy"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCheckRejectsAnEnabledClientRouteWithoutItsAccountToken(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude-account", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	addAccountProfile(&cfg, "codex", "codex-account", "Codex", configuration.Endpoints{OpenAIResponses: "https://codex.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "claude-token"); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "check")
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
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "stale", "stale-account", "Stale", configuration.Endpoints{OpenAIResponses: "https://stale.test/v1"}, configuration.ClientCodex, "stale-model")
	addAccountProfile(&cfg, "claude", "claude-account", "Claude", configuration.Endpoints{Anthropic: "https://claude.test"}, configuration.ClientClaude, "claude-test")
	addAccountProfile(&cfg, "codex", "codex-account", "Codex", configuration.Endpoints{OpenAIResponses: "https://codex.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(codexTarget, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{codexTarget}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	codexRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(codexTarget, codexRuntime); err != nil {
		t.Fatal(err)
	}
	for account, token := range map[string]string{"claude-account": "claude-token", "codex-account": "codex-token"} {
		if err := secretStore.Set(account, token); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]int{}
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		seen[req.URL.Host]++
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
	}

	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	if seen["claude.test"] != 1 || seen["codex.test"] != 1 || seen["stale.test"] != 0 {
		t.Fatalf("probed endpoints = %#v", seen)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Codex") || strings.Contains(out.String(), "Stale") {
		t.Fatalf("check output does not describe active client Routes:\n%s", out.String())
	}
}

func TestCheckUsesBoundedAuthenticationStabilityWithoutMutation(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []int
		wantError  bool
		wantText   []string
		rejectText []string
	}{
		{
			name:       "healthy first observation",
			statuses:   []int{http.StatusOK},
			wantText:   []string{"Claude", "Ready", "Every enabled client route is healthy"},
			rejectText: []string{"transient response", "aigw rotate"},
		},
		{
			name:       "recovered transient",
			statuses:   []int{http.StatusUnauthorized, http.StatusOK, http.StatusOK, http.StatusOK},
			wantText:   []string{"Claude", "Ready", "Claude authentication recovered after a transient response", "Every enabled client route is healthy"},
			rejectText: []string{"aigw rotate"},
		},
		{
			name:      "persistent invalid token",
			statuses:  []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
			wantError: true,
			wantText:  []string{"API token is invalid or belongs to a different gateway", "aigw rotate"},
		},
		{
			name:       "unstable authentication",
			statuses:   []int{http.StatusUnauthorized, http.StatusOK, http.StatusUnauthorized, http.StatusOK},
			wantError:  true,
			wantText:   []string{"Authentication could not be confirmed consistently", "aigw check"},
			rejectText: []string{"aigw rotate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, out, secretStore, _ := testApp(t, "")
			cfg := configuration.NewConfig()
			addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test"}, configuration.ClientClaude, "claude-test")
			cfg.Routes[configuration.ClientClaude] = "dmx"
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("dmx", "stable-token"); err != nil {
				t.Fatal(err)
			}
			beforeConfig, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}
			prompt := &recordingPrompt{}
			app.Interactive = true
			app.Prompt = prompt
			calls := 0
			app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
				if calls >= len(tt.statuses) {
					t.Fatalf("unexpected authentication probe %d", calls+1)
				}
				status := tt.statuses[calls]
				calls++
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"message":"bounded observation"}`)), Request: req}, nil
			}

			err = execute(t, app, "check")
			if (err != nil) != tt.wantError {
				t.Fatalf("check error = %v, wantError=%v\n%s", err, tt.wantError, out.String())
			}
			if calls != len(tt.statuses) {
				t.Fatalf("probe calls = %d, want %d", calls, len(tt.statuses))
			}
			for _, want := range tt.wantText {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("check output lacks %q:\n%s", want, out.String())
				}
			}
			for _, reject := range tt.rejectText {
				if strings.Contains(out.String(), reject) {
					t.Fatalf("check output unexpectedly contains %q:\n%s", reject, out.String())
				}
			}
			if prompt.calls != 0 {
				t.Fatalf("check prompted %d times", prompt.calls)
			}
			afterConfig, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}
			if string(afterConfig) != string(beforeConfig) {
				t.Fatalf("check changed configuration\nbefore:\n%s\nafter:\n%s", beforeConfig, afterConfig)
			}
			if token, err := secretStore.Get("dmx"); err != nil || token != "stable-token" {
				t.Fatalf("stored token = %q, %v; want unchanged", token, err)
			}
		})
	}
}

func TestCheckIdentifiesExternalLoopbackTransportWithoutClaimingOwnership(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "local", "local", "Local Compatibility Layer", configuration.Endpoints{Anthropic: "http://127.0.0.1:4567"}, configuration.ClientClaude, "model-test")
	cfg.Routes[configuration.ClientClaude] = "local"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("local", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Claude", "external loopback compatibility layer that AIGW does not manage"} {
		if !strings.Contains(text, want) {
			t.Fatalf("check lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "4567") {
		t.Fatalf("check exposed the loopback transport port:\n%s", text)
	}
}

func TestCheckDoesNotDescribeRemoteHTTPSAsExternalLoopbackTransport(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "remote", "remote", "Remote Gateway", configuration.Endpoints{Anthropic: "https://gateway.test"}, configuration.ClientClaude, "model-test")
	cfg.Routes[configuration.ClientClaude] = "remote"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("remote", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "External loopback compatibility layer") {
		t.Fatalf("check misclassified remote endpoint:\n%s", out.String())
	}
}

func TestCheckRejectsLocalProgramBuildBeforeClaimingHealth(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Version = "0.1.0-rc.44+local.test"
	err := execute(t, app, "check")
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
	app, out, _, _ := testApp(t, "")
	app.Version = "0.1.0-dev"
	err := execute(t, app, "check")
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
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["future"] = configuration.Account{
		Label:        "Future Gateway",
		Endpoints:    configuration.Endpoints{Anthropic: "https://future.test"},
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "future", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("future", "test-token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Ready") || strings.Contains(out.String(), "aigw balance") {
		t.Fatalf("check output = %s", out.String())
	}
}
