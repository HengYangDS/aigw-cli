package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
		{name: "disabled Codex adapter", args: []string{"verify", "--for", "codex"}, prep: func(app *cli.App) {
			saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		}, want: "Codex adapter is disabled"},
		{name: "missing Claude token", args: []string{"verify", "--for", "claude"}, prep: func(app *cli.App) {
			saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
		}, want: "Token for account"},
		{name: "all with unresolved route", args: []string{"verify", "--for", "all"}, prep: func(app *cli.App) {
			cfg := configuration.NewConfig()
			cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test", OpenAIResponses: "https://one.test/v1"}}
			cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "one", Client: configuration.ClientClaude, Model: "claude-test"}
			cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "one", Client: configuration.ClientCodex, Model: "gpt-test"}
			cfg.Routes[configuration.ClientClaude] = "claude"
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
		}, want: `no route selected for client "codex"`},
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

func TestVerifyCodexRunsTheConfiguredClientOnceAndReportsItsIdentity(t *testing.T) {
	app, out, _, runner := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	executable := executableFixture(t, "codex")
	root := t.TempDir()
	targets := []string{
		filepath.Join(root, "z", "config.toml"),
		filepath.Join(root, "a", "config.toml"),
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := codex.SyncConfig(target, runtime); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executable, Targets: targets}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	httpRequests := 0
	app.HTTP = &fakeHTTP{status: 200, handler: func(req *http.Request) (*http.Response, error) {
		httpRequests++
		return nil, fmt.Errorf("unexpected HTTP request to %s", req.URL)
	}}

	if err := execute(t, app, "verify", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if httpRequests != 0 {
		t.Fatalf("HTTP requests = %d, want 0", httpRequests)
	}
	if len(runner.plans) != 2 || !slices.Equal(runner.plans[0].Args, []string{"--version"}) {
		t.Fatalf("plans = %#v", runner.plans)
	}
	execPlan := runner.plans[1]
	if execPlan.Executable != executable || planArgumentValue(execPlan.Args, "--output-last-message") == "" || planArgumentValue(execPlan.Args, "--model") != "gpt-test" {
		t.Fatalf("Codex execution plan = %#v", execPlan)
	}
	if got := planEnvironmentValue(execPlan.Env, "CODEX_HOME"); got != filepath.Dir(targets[1]) {
		t.Fatalf("CODEX_HOME = %q", got)
	}
	executableBytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	wantSHA256 := fmt.Sprintf("%x", sha256.Sum256(executableBytes))
	if strings.Contains(out.String(), "verify-token") || strings.Contains(out.String(), "AIGW_OK") || !strings.Contains(out.String(), "codex-cli 0.0.0-test") || !strings.Contains(out.String(), wantSHA256) {
		t.Fatalf("verify output = %s", out.String())
	}
}

func TestVerifyCodexReportsTheClientFailureAndOneRetryAction(t *testing.T) {
	app, out, _, runner := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{
		Label: "DMX",
		Endpoints: configuration.Endpoints{
			OpenAIResponses: "https://example.test/v1",
		},
	}
	cfg.Profiles["gpt"] = configuration.Profile{
		Label:   "GPT",
		Account: "dmx",
		Client:  configuration.ClientCodex,
		Model:   "gpt-test",
	}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
		Enabled:    true,
		Executable: executableFixture(t, "codex"),
		Targets:    []string{target},
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runner.output = []byte("Error loading config.toml: unknown configuration field mcp_servers.github.disabled_reason\n")
	runner.capture = errors.New("exit status 1")

	err = execute(t, app, "verify", "--for", "codex")
	if err == nil {
		t.Fatal("failed Codex verification was accepted")
	}
	text := out.String()
	for _, want := range []string{
		"Codex minimal verification request failed",
		"unknown configuration field mcp_servers.github.disabled_reason",
		"aigw verify --for codex",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("verification output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "aigw check") {
		t.Fatalf("verification output retained the unrelated check loop:\n%s", text)
	}
}

func TestVerifyInfersClientFromExplicitProfile(t *testing.T) {
	app, _, _, runner := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	requests := 0
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(req *http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("unexpected HTTP request to %s", req.URL)
	}}

	if err := execute(t, app, "verify", "--profile", "gpt"); err != nil {
		t.Fatal(err)
	}
	if requests != 0 || len(runner.plans) != 2 {
		t.Fatalf("requests = %d, plans = %#v", requests, runner.plans)
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
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{codexTarget}}
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
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s", req.URL)
		return nil, nil
	}}

	if err := execute(t, app, "verify", "--for", "all"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 3 {
		t.Fatalf("verification plans = %#v", runner.plans)
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
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{codexTarget}}
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
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s", req.URL)
		return nil, nil
	}}
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
	app, _, _, runner := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runner.output = []byte("wrong\n")
	err = execute(t, app, "verify", "--for", "codex")
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
