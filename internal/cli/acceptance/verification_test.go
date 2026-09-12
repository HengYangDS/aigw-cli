package cli_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/codex"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

func TestVerifyClaudeUsesManagedProcessBoundary(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude Fable", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-fable-5"}
	cfg.Routes[configuration.ClientClaude] = "claude-fable-5"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"verify", "--for", "claude"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != claudeExecutable || !strings.Contains(strings.Join(runner.plans[0].Args, " "), "AIGW_OK") {
		t.Fatalf("Claude verify plan = %#v", runner.plans)
	}
	wantArgs := []string{"--bare", "--settings", app.ClaudeSettingsPath, "--disable-slash-commands", "--no-session-persistence", "--tools", "", "--print", "Reply with exactly: AIGW_OK"}
	if got := runner.plans[0].Args; !slices.Equal(got, wantArgs) {
		t.Fatalf("Claude verification must consume the synchronized settings, got %#v", got)
	}
	if strings.Contains(out.String(), "verify-token") || !strings.Contains(out.String(), "Live protocol verification") {
		t.Fatalf("verify output = %s", out.String())
	}
}

func TestVerifyAllRequiresSynchronizedClientAdapters(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"verify", "--for", "all"})
	if err == nil || !strings.Contains(err.Error(), "Full verification requires a ready Codex adapter") || !strings.Contains(err.Error(), "Codex adapter is disabled") {
		t.Fatalf("error = %v", err)
	}
	if _, checkpointErr := app.Config.LoadVerifiedCheckpoint(); checkpointErr == nil {
		t.Fatal("verification checkpoint was written despite failed readiness preflight")
	}
}

func TestVerifyAdmitsTargetArgumentsBeforeReadingConfiguration(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"missing target", []string{"verify"}, "choose a verification target"},
		{"empty client", []string{"verify", "--for="}, "choose a verification target"},
		{"empty profile", []string{"verify", "--profile="}, "choose a verification target"},
		{"unknown client", []string{"verify", "--for", "unknown"}, "--for must be"},
		{"conflicting targets", []string{"verify", "--for", "codex", "--profile", "one"}, "[for profile] were all set"},
		{"profile with all", []string{"verify", "--for", "all", "--profile", "one"}, "[for profile] were all set"},
		{"explicit empty conflict", []string{"verify", "--for=", "--profile", "one"}, "[for profile] were all set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, runner, httpClient := testApp(t, "")
			const original = "invalid configuration ["
			if err := os.WriteFile(app.Config.Path(), []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			err := cli.Execute(app, test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want argument diagnostic %q", err, test.want)
			}
			if got, err := os.ReadFile(app.Config.Path()); err != nil || string(got) != original {
				t.Fatalf("configuration = %q, error = %v", got, err)
			}
			entries, err := os.ReadDir(filepath.Dir(app.Config.Path()))
			if err != nil || len(entries) != 1 {
				t.Fatalf("invalid arguments changed configuration storage: %v, %v", entries, err)
			}
			if len(runner.plans) != 0 || httpClient.calls != 0 {
				t.Fatal("invalid arguments invoked a client or a provider")
			}
		})
	}
}

func TestVerifyRejectsUnavailableConfigurationAndClientState(t *testing.T) {
	tests := []struct {
		name string
		args []string
		prep func(*cli.App)
		want string
	}{
		{name: "config load", args: []string{"verify", "--for", "codex"}, prep: func(app *cli.App) { app.Config = configuration.NewStore(t.TempDir()) }, want: "read config"},
		{name: "unknown profile", args: []string{"verify", "--profile", "missing"}, prep: func(app *cli.App) {
			saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		}, want: "unknown profile"},
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
			synchronizeClaudeProjection(t, app, cfg)
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
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
		}, want: `no route selected for client "codex"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			if test.prep != nil {
				test.prep(app)
			}
			err := cli.Execute(app, test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestVerifyCodexRunsTheConfiguredClientOnceAndReportsItsIdentity(t *testing.T) {
	app, out, _, runner, _ := testApp(t, "")
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
		runtime.CredentialCommand = app.Executable
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

	if err := cli.Execute(app, []string{"verify", "--for", "codex"}); err != nil {
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
	app, out, _, runner, _ := testApp(t, "")
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
	runtime.CredentialCommand = app.Executable
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

	err = cli.Execute(app, []string{"verify", "--for", "codex"})
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
	app, _, _, runner, _ := testApp(t, "")
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
	runtime.CredentialCommand = app.Executable
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

	if err := cli.Execute(app, []string{"verify", "--profile", "gpt"}); err != nil {
		t.Fatal(err)
	}
	if requests != 0 || len(runner.plans) != 2 {
		t.Fatalf("requests = %d, plans = %#v", requests, runner.plans)
	}
}

func readyVerificationApp(t *testing.T) (*cli.App, *fakeRunner) {
	t.Helper()
	app, _, secretStore, runner, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(codexTarget, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{codexTarget}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s", req.URL)
		return nil, nil
	}}
	return app, runner
}

type verificationCompletionRunner struct {
	*fakeRunner
	completed func()
}

func (runner verificationCompletionRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	output, err := runner.fakeRunner.RunCapture(ctx, plan)
	if err == nil && slices.Contains(plan.Args, "--output-last-message") {
		runner.completed()
	}
	return output, err
}

func TestVerifyAllPreservesConfigurationChangedDuringLiveRequest(t *testing.T) {
	app, runner := readyVerificationApp(t)
	var newCheckpoint []byte
	app.Runner = verificationCompletionRunner{fakeRunner: runner, completed: func() {
		cfg, err := app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		profile := cfg.Profiles["gpt"]
		profile.Model = "newer-model"
		cfg.Profiles["gpt"] = profile
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := app.Config.SaveVerifiedCheckpoint(t.Context(), cfg, configuration.AdmittedClientIDs()); err != nil {
			t.Fatal(err)
		}
		newCheckpoint, err = os.ReadFile(app.Config.Path() + ".verified.json")
		if err != nil {
			t.Fatal(err)
		}
	}}
	if err := cli.Execute(app, []string{"verify", "--for", "all"}); err == nil || !strings.Contains(err.Error(), "configuration changed") {
		t.Fatalf("stale live verification was accepted: %v", err)
	}
	current, err := app.Config.Load()
	if err != nil || current.Profiles["gpt"].Model != "newer-model" {
		t.Fatalf("newer configuration was lost: %v", err)
	}
	checkpoint, err := os.ReadFile(app.Config.Path() + ".verified.json")
	if err != nil || !slices.Equal(checkpoint, newCheckpoint) {
		t.Fatalf("newer verification checkpoint was changed: %v", err)
	}
}

func TestVerifyAllWritesVerifiedCheckpoint(t *testing.T) {
	app, runner := readyVerificationApp(t)
	if err := cli.Execute(app, []string{"verify", "--for", "all"}); err != nil {
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
	app, runner := readyVerificationApp(t)
	checkpoint := app.Config.Path() + ".verified.json"
	if err := os.Mkdir(checkpoint, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkpoint, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"verify", "--for", "all"}); err == nil {
		t.Fatal("checkpoint write failure was accepted")
	}
	if len(runner.plans) != 3 {
		t.Fatalf("failure occurred before live verification: %#v", runner.plans)
	}
}

func TestVerifyRejectsMissingResponseSentinel(t *testing.T) {
	app, _, _, runner, _ := testApp(t, "")
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
	runtime.CredentialCommand = app.Executable
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "codex"), Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runner.output = []byte("wrong\n")
	err = cli.Execute(app, []string{"verify", "--for", "codex"})
	if err == nil || !strings.Contains(err.Error(), "did not return the expected AIGW_OK verification marker") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyClaudeRejectsMissingResponseSentinel(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
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
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	runner.output = []byte("wrong response\n")
	err := cli.Execute(app, []string{"verify", "--for", "claude"})
	if err == nil || !strings.Contains(err.Error(), "did not return the expected AIGW_OK verification marker") {
		t.Fatalf("error = %v", err)
	}
}
