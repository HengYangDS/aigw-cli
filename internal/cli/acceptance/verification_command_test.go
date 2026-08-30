package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyCommandRejectsInvalidInputsAndMissingState(t *testing.T) {
	tests := []struct {
		name string
		args []string
		prep func(*cli.App)
		want string
	}{
		{name: "unknown client", args: []string{"verify", "--for", "bogus"}, want: "--for must be"},
		{name: "profile with client", args: []string{"verify", "--for", "codex", "--profile", "one"}, want: "choose either --profile or --for, not both"},
		{name: "profile with all", args: []string{"verify", "--for", "all", "--profile", "one"}, want: "choose either --profile or --for, not both"},
		{name: "config load", args: []string{"verify", "--for", "codex"}, prep: func(app *cli.App) { app.Config = configuration.NewStore(t.TempDir()) }, want: "read config"},
		{name: "unknown profile", args: []string{"verify", "--profile", "missing"}, prep: func(app *cli.App) {
			saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		}, want: "unknown profile"},
		{name: "missing target", args: []string{"verify"}, prep: func(app *cli.App) {
			saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		}, want: "--for must be"},
		{name: "missing token", args: []string{"verify", "--for", "codex"}, prep: func(app *cli.App) {
			saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		}, want: "is unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _ := testApp(t, "")
			if test.prep != nil {
				test.prep(app)
			}
			err := execute(t, app, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestVerifyCodexPerformsBoundedResponsesRequest(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	var requestBody string
	app.HTTP = &fakeHTTP{status: 200, handler: func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/responses" {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		data, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = string(data)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"AIGW_OK"}]}]}`)), Request: req}, nil
	}}

	if err := execute(t, app, "verify", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestBody, `"model":"gpt-test"`) || !strings.Contains(requestBody, `"store":false`) || !strings.Contains(requestBody, "AIGW_OK") {
		t.Fatalf("verify body = %s", requestBody)
	}
	if strings.Contains(out.String(), "verify-token") || !strings.Contains(out.String(), "Live protocol verification") {
		t.Fatalf("verify output = %s", out.String())
	}
}

func TestVerifyInfersClientFromExplicitProfile(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	requests := 0
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output_text":"AIGW_OK"}`)), Request: req}, nil
	}}

	if err := execute(t, app, "verify", "--profile", "gpt"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestVerifyAllWritesVerifiedCheckpoint(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
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
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, body: `{"status":"completed","output_text":"AIGW_OK"}`}

	if err := execute(t, app, "verify", "--for", "all"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 {
		t.Fatalf("Claude verification plans = %#v", runner.plans)
	}
	checkpoint, err := app.Config.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoint.Clients) != 2 || checkpoint.Config.Routes[configuration.ClientCodex] != "gpt" {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
}

func TestVerifyAllReturnsCheckpointWriteFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
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
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, body: `{"status":"completed","output_text":"AIGW_OK"}`}
	checkpoint := app.Config.Path() + ".verified.json"
	if err := os.Mkdir(checkpoint, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkpoint, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "verify", "--for", "all"); err == nil {
		t.Fatal("checkpoint write failure was accepted")
	}
}

func TestVerifyRejectsMissingResponseSentinel(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, body: `{"status":"completed","output_text":"not-the-sentinel"}`}
	err := execute(t, app, "verify", "--for", "codex")
	if err == nil || !strings.Contains(err.Error(), "did not return the expected AIGW_OK verification marker") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyClaudeRejectsMissingResponseSentinel(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	runner.output = []byte("wrong response\n")
	err := execute(t, app, "verify", "--for", "claude")
	if err == nil || !strings.Contains(err.Error(), "did not return the expected AIGW_OK verification marker") {
		t.Fatalf("error = %v", err)
	}
}
