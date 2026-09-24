package cli_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestStatusAndCheckHideUnreadableConfigDetails(t *testing.T) {
	for _, command := range [][]string{{"status"}, {"check"}} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			if err := os.WriteFile(app.Config.Path(), []byte("version = [\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			err := cli.Execute(app, command)
			if err == nil {
				t.Fatalf("%s unexpectedly succeeded", strings.Join(command, " "))
			}
			text := out.String()
			for _, want := range []string{"Cannot read or validate local configuration", "aigw doctor"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s output lacks %q:\n%s", strings.Join(command, " "), want, text)
				}
			}
			for _, forbidden := range []string{"parse config:", "validate config:", "version = [", app.Config.Path()} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s leaked %q:\n%s", strings.Join(command, " "), forbidden, text)
				}
			}
		})
	}
}

type canonicalReadinessDocument struct {
	Clients map[string]struct {
		State      string `json:"state"`
		NextAction string `json:"next_action"`
	} `json:"clients"`
}

func TestExternalCredentialPolicyDoesNotRequireAnAIGWToken(t *testing.T) {
	app, out, _, runner, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://example.invalid"}}
	cfg.Routes["claude"] = qualifiedRoute("Claude", "gateway", "fixture", configuration.ProtocolAnthropic)
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	command := filepath.Join(t.TempDir(), "credential adapter")
	cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
	binding := cfg.Clients[configuration.ClientClaude]
	binding.CredentialCommand = command
	cfg.Clients[configuration.ClientClaude] = binding
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: errors.New("native metadata must not select external credentials")}
	for _, args := range [][]string{{"sync"}, {"check", "--json"}, {"status", "--json"}, {"route", "list"}, {"route", "show", "claude"}, {"use", "--for", "claude", "claude"}, {"client", "disable", "claude"}, {"client", "enable", "claude", "--executable", cfg.Clients[configuration.ClientClaude].Executable}} {
		out.Reset()
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v required a native Token: %v", args, err)
		}
		if args[0] == "check" || args[0] == "status" {
			var result canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			state := result.Clients[configuration.ClientClaude]
			if state.State != "configured" || state.NextAction != "aigw verify --for claude" {
				t.Fatalf("external credential readiness = %#v", state)
			}
		}
	}
	if len(runner.plans) != 0 {
		t.Fatal("local admission executed a client or credential helper")
	}
	if err := cli.Execute(app, []string{"test", "--for", "claude"}); err == nil || !strings.Contains(err.Error(), "external credential helper") {
		t.Fatalf("direct endpoint test did not explain external credential ownership: %v", err)
	}
}

func TestCheckClassifiesAuthenticatedProbeOutcomes(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantState string
		wantError bool
	}{
		{name: "endpoint checked", status: http.StatusOK, wantState: "endpoint_checked"},
		{name: "invalid token", status: http.StatusUnauthorized, body: `{"message":"invalid token"}`, wantState: "invalid", wantError: true},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"slow down"}`, wantState: "degraded", wantError: true},
		{name: "unexpected response", status: http.StatusTeapot, body: `{"message":"unexpected"}`, wantState: "unavailable", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, _, httpClient := testApp(t, "")
			app.Version = "1.0.0"
			cfg := configuration.NewConfig()
			addAccountRoute(
				&cfg,
				"claude",
				"team",
				"Team",
				configuration.Endpoints{Anthropic: "https://team.test"},
				configuration.ClientClaude,
				"claude-test",
			)
			cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
			cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "token"); err != nil {
				t.Fatal(err)
			}
			httpClient.status = test.status
			httpClient.body = test.body

			err := cli.Execute(app, []string{"check", "--json", "--endpoint-only"})
			if (err != nil) != test.wantError {
				t.Fatalf("check error = %v, want error %v\n%s", err, test.wantError, out.String())
			}
			var document canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("decode check JSON: %v\n%s", err, out.String())
			}
			state := document.Clients[configuration.ClientClaude].State
			if state != test.wantState {
				t.Fatalf("check state = %q, want %q\n%s", state, test.wantState, out.String())
			}
		})
	}
}

func TestReadinessDistinguishesConfigurationFromEndpointCheck(t *testing.T) {
	for _, command := range []string{"status", "doctor"} {
		t.Run(command, func(t *testing.T) {
			app, out, secretStore, _, _ := testApp(t, "")
			app.Version = "1.0.0"
			cfg := configuration.NewConfig()
			addAccountRoute(
				&cfg,
				"claude",
				"team",
				"Team",
				configuration.Endpoints{Anthropic: "https://team.test"},
				configuration.ClientClaude,
				"claude-test",
			)
			cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
			cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "token"); err != nil {
				t.Fatal(err)
			}

			if err := cli.Execute(app, []string{command, "--json"}); err != nil {
				t.Fatalf("%s failed: %v\n%s", command, err, out.String())
			}
			var document canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("decode %s JSON: %v\n%s", command, err, out.String())
			}
			state := document.Clients[configuration.ClientClaude].State
			if state != "configured" {
				t.Fatalf("%s state = %q, want configured\n%s", command, state, out.String())
			}
		})
	}

	app, out, secretStore, _, _ := testApp(t, "")
	app.Version = "1.0.0"
	cfg := configuration.NewConfig()
	addAccountRoute(
		&cfg,
		"claude",
		"team",
		"Team",
		configuration.Endpoints{Anthropic: "https://team.test"},
		configuration.ClientClaude,
		"claude-test",
	)
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"check", "--json", "--endpoint-only"}); err != nil {
		t.Fatalf("check failed: %v\n%s", err, out.String())
	}
	var document canonicalReadinessDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("decode check JSON: %v\n%s", err, out.String())
	}
	state := document.Clients[configuration.ClientClaude].State
	if state != "endpoint_checked" {
		t.Fatalf("check state = %q, want endpoint_checked\n%s", state, out.String())
	}
}

func TestReadOnlyCommandsShareDeferredClientState(t *testing.T) {
	commands := []string{"status", "check", "doctor"}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			app.Version = "1.0.0"
			cfg := configuration.NewConfig()
			addAccountRoute(
				&cfg,
				"claude",
				"team",
				"Team",
				configuration.Endpoints{Anthropic: "https://team.test"},
				configuration.ClientClaude,
				"claude-test",
			)
			cfg.SetRecommendedRoute(configuration.ClientClaude, "claude")
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}

			if err := cli.Execute(app, []string{command, "--json"}); err != nil {
				t.Fatalf("%s failed for a deferred capability: %v\n%s", command, err, out.String())
			}
			var document canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("decode %s JSON: %v\n%s", command, err, out.String())
			}
			client := document.Clients[configuration.ClientClaude]
			state, action := client.State, client.NextAction
			if state != "deferred" || action != "aigw use --for claude claude" {
				t.Fatalf("%s state = %q, next_action = %q; want deferred and an explicit Claude selection\n%s", command, state, action, out.String())
			}
		})
	}
}

func TestStatusAndDoctorObserveCredentialMetadataWithoutSideEffects(t *testing.T) {
	for _, command := range []string{"status", "doctor"} {
		t.Run(command, func(t *testing.T) {
			app, _, secretStore, runner, httpClient := testApp(t, "")
			cfg := configuration.NewConfig()
			addAccountRoute(
				&cfg,
				"claude",
				"team",
				"Team",
				configuration.Endpoints{Anthropic: "https://team.test"},
				configuration.ClientClaude,
				"claude-test",
			)
			cfg.Accounts["team"] = configuration.Account{
				Label:        "Team",
				Endpoints:    configuration.Endpoints{Anthropic: "https://team.test"},
				AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"},
			}
			cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
			cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "never-read-this-token"); err != nil {
				t.Fatal(err)
			}
			diagnosticCredentialStore := app.Accounts
			if err := diagnosticCredentialStore.Set("team", secrets.DiagnosticCredential{SystemToken: "never-read-this-system-token", UserID: "never-read-this-user"}); err != nil {
				t.Fatal(err)
			}
			before := readFile(t, app.Config.Path())
			beforeFiles := directoryNames(t, filepath.Dir(app.Config.Path()))

			tokenObservation := &recordingCredentialStore[string]{backend: secretStore}
			diagnosticObservation := &recordingCredentialStore[secrets.DiagnosticCredential]{backend: diagnosticCredentialStore}
			app.Secrets = tokenObservation
			app.Accounts = diagnosticObservation
			prompt := &scriptedPrompt{}
			app.Interactive = true
			app.Prompt = prompt

			if err := cli.Execute(app, []string{command}); err != nil {
				t.Fatal(err)
			}
			if len(tokenObservation.getCalls) != 0 || len(diagnosticObservation.getCalls) != 0 {
				t.Fatalf("%s read credential values: Token=%q diagnostic=%q", command, tokenObservation.getCalls, diagnosticObservation.getCalls)
			}
			if len(tokenObservation.existsCalls) == 0 {
				t.Fatalf("%s did not observe Token metadata", command)
			}
			if command == "status" && (len(diagnosticObservation.existsCalls) != 1 || diagnosticObservation.existsCalls[0] != "team") {
				t.Fatalf("status diagnostic credential observations = %q", diagnosticObservation.existsCalls)
			}
			if len(tokenObservation.setCalls) != 0 || len(tokenObservation.deleteCalls) != 0 || len(diagnosticObservation.setCalls) != 0 || len(diagnosticObservation.deleteCalls) != 0 {
				t.Fatalf("%s mutated credentials: Token set=%q delete=%q; diagnostic set=%q delete=%q", command, tokenObservation.setCalls, tokenObservation.deleteCalls, diagnosticObservation.setCalls, diagnosticObservation.deleteCalls)
			}
			if prompt.callCount() != 0 {
				t.Fatalf("%s prompted %d times", command, prompt.callCount())
			}
			if len(runner.plans) != 0 {
				t.Fatalf("%s started a client process: %#v", command, runner.plans)
			}
			if calls := httpClient.calls; calls != 0 {
				t.Fatalf("%s performed %d HTTP requests", command, calls)
			}
			if after := readFile(t, app.Config.Path()); string(after) != string(before) {
				t.Fatalf("%s changed configuration bytes", command)
			}
			if afterFiles := directoryNames(t, filepath.Dir(app.Config.Path())); strings.Join(afterFiles, "\x00") != strings.Join(beforeFiles, "\x00") {
				t.Fatalf("%s changed configuration directory entries: before=%q after=%q", command, beforeFiles, afterFiles)
			}
		})
	}
}

func TestStatusReportsDiagnosticMetadataFailureWithOneSafeAction(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountRoute(
		&cfg,
		"claude",
		"team",
		"Team",
		configuration.Endpoints{Anthropic: "https://team.test"},
		configuration.ClientClaude,
		"claude-test",
	)
	cfg.Accounts["team"] = configuration.Account{
		Label:        "Team",
		Endpoints:    configuration.Endpoints{Anthropic: "https://team.test"},
		AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"},
	}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "never-read-this-token"); err != nil {
		t.Fatal(err)
	}
	diagnosticCredentialStore, err := secrets.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential metadata unavailable")
	observed := &recordingCredentialStore[secrets.DiagnosticCredential]{backend: diagnosticCredentialStore, existsErr: want}
	app.Accounts = observed

	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	if len(observed.getCalls) != 0 {
		t.Fatalf("status read diagnostic credential values %q", observed.getCalls)
	}
	if !strings.Contains(out.String(), "Credential metadata unavailable") || strings.Count(out.String(), "aigw doctor") != 1 || strings.Contains(out.String(), "aigw check") {
		t.Fatalf("status did not present one safe metadata recovery action:\n%s", out.String())
	}
}
