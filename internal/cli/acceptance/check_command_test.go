package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckSurfacesMissingDefaultRuntimeAndToken(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
	err := execute(t, app, "check")
	if err == nil || !strings.Contains(err.Error(), "System secret is missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckProvidesOneClearHealthSummary(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	cfg.Adapters["claude"] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Configuration file", "System secret", "Gateway", "Authentication healthy", "Everything is healthy"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check lacks %q:\n%s", want, out.String())
		}
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
			wantText:   []string{"Authentication healthy", "Everything is healthy"},
			rejectText: []string{"transient response", "aigw rotate"},
		},
		{
			name:       "recovered transient",
			statuses:   []int{http.StatusUnauthorized, http.StatusOK, http.StatusOK, http.StatusOK},
			wantText:   []string{"Authentication healthy", "Authentication recovered after a transient response", "Everything is healthy"},
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
			addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
			cfg.Routes.Default = "dmx"
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
	addAccountProfile(&cfg, "local", "local", "Local Compatibility Layer", configuration.Endpoints{OpenAIResponses: "http://127.0.0.1:4567/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "model-test"})
	cfg.Routes.Default = "local"
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
	for _, want := range []string{"Transport", "External loopback compatibility layer", "Codex requests use the external listener", "AIGW does not start, stop, or configure it"} {
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
	addAccountProfile(&cfg, "remote", "remote", "Remote Gateway", configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "model-test"})
	cfg.Routes.Default = "remote"
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
		Endpoints:    configuration.Endpoints{OpenAIResponses: "https://future.test/v1"},
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "future", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("future", "test-token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "This version does not provide diagnostics for this provider") || strings.Contains(out.String(), "aigw balance") {
		t.Fatalf("check output = %s", out.String())
	}
}
