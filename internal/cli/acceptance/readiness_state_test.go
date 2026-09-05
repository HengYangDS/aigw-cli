package cli_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

type canonicalReadinessDocument struct {
	Clients map[string]struct {
		State      string `json:"state"`
		NextAction string `json:"next_action"`
	} `json:"clients"`
}

func TestCheckClassifiesAuthenticatedProbeOutcomes(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantState string
		wantError bool
	}{
		{name: "ready", status: http.StatusOK, wantState: "ready"},
		{name: "invalid token", status: http.StatusUnauthorized, body: `{"message":"invalid token"}`, wantState: "invalid", wantError: true},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"slow down"}`, wantState: "degraded", wantError: true},
		{name: "unexpected response", status: http.StatusTeapot, body: `{"message":"unexpected"}`, wantState: "unavailable", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, _ := testApp(t, "")
			app.Version = "1.0.0"
			cfg := configuration.NewConfig()
			addAccountProfile(
				&cfg,
				"claude",
				"team",
				"Team",
				configuration.Endpoints{Anthropic: "https://team.test"},
				configuration.ClientClaude,
				"claude-test",
			)
			cfg.Routes[configuration.ClientClaude] = "claude"
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{
				Enabled:    true,
				Executable: executableFixture(t, "claude"),
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "token"); err != nil {
				t.Fatal(err)
			}
			app.HTTP.(*fakeHTTP).status = test.status
			app.HTTP.(*fakeHTTP).body = test.body

			err := execute(t, app, "check", "--json")
			if (err != nil) != test.wantError {
				t.Fatalf("check error = %v, want error %v\n%s", err, test.wantError, out.String())
			}
			var document canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("decode check JSON: %v\n%s", err, out.String())
			}
			state, _ := canonicalClientState(document, configuration.ClientClaude)
			if state != test.wantState {
				t.Fatalf("check state = %q, want %q\n%s", state, test.wantState, out.String())
			}
		})
	}
}

func TestReadinessDistinguishesConfiguredFromProvenReady(t *testing.T) {
	for _, command := range []string{"status", "doctor"} {
		t.Run(command, func(t *testing.T) {
			app, out, secretStore, _ := testApp(t, "")
			app.Version = "1.0.0"
			cfg := configuration.NewConfig()
			addAccountProfile(
				&cfg,
				"claude",
				"team",
				"Team",
				configuration.Endpoints{Anthropic: "https://team.test"},
				configuration.ClientClaude,
				"claude-test",
			)
			cfg.Routes[configuration.ClientClaude] = "claude"
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{
				Enabled:    true,
				Executable: executableFixture(t, "claude"),
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "token"); err != nil {
				t.Fatal(err)
			}

			if err := execute(t, app, command, "--json"); err != nil {
				t.Fatalf("%s failed: %v\n%s", command, err, out.String())
			}
			var document canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("decode %s JSON: %v\n%s", command, err, out.String())
			}
			state, _ := canonicalClientState(document, configuration.ClientClaude)
			if state != "configured" {
				t.Fatalf("%s state = %q, want configured\n%s", command, state, out.String())
			}
		})
	}

	app, out, secretStore, _ := testApp(t, "")
	app.Version = "1.0.0"
	cfg := configuration.NewConfig()
	addAccountProfile(
		&cfg,
		"claude",
		"team",
		"Team",
		configuration.Endpoints{Anthropic: "https://team.test"},
		configuration.ClientClaude,
		"claude-test",
	)
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{
		Enabled:    true,
		Executable: executableFixture(t, "claude"),
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check", "--json"); err != nil {
		t.Fatalf("check failed: %v\n%s", err, out.String())
	}
	var document canonicalReadinessDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("decode check JSON: %v\n%s", err, out.String())
	}
	state, _ := canonicalClientState(document, configuration.ClientClaude)
	if state != "ready" {
		t.Fatalf("check state = %q, want ready\n%s", state, out.String())
	}
}

func TestReadOnlyCommandsShareDeferredClientState(t *testing.T) {
	commands := []string{"status", "check", "doctor"}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			app, out, _, _ := testApp(t, "")
			app.Version = "1.0.0"
			cfg := configuration.NewConfig()
			addAccountProfile(
				&cfg,
				"claude",
				"team",
				"Team",
				configuration.Endpoints{Anthropic: "https://team.test"},
				configuration.ClientClaude,
				"claude-test",
			)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}

			if err := execute(t, app, command, "--json"); err != nil {
				t.Fatalf("%s failed for a deferred capability: %v\n%s", command, err, out.String())
			}
			var document canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("decode %s JSON: %v\n%s", command, err, out.String())
			}
			state, action := canonicalClientState(document, configuration.ClientClaude)
			if state != "deferred" || action != "aigw use claude" {
				t.Fatalf("%s state = %q, next_action = %q; want deferred and aigw use claude\n%s", command, state, action, out.String())
			}
		})
	}
}

func TestStatusAndDoctorObserveCredentialMetadataWithoutSideEffects(t *testing.T) {
	for _, command := range []string{"status", "doctor"} {
		t.Run(command, func(t *testing.T) {
			app, _, secretStore, runner := testApp(t, "")
			cfg := configuration.NewConfig()
			addAccountProfile(
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
			cfg.Routes[configuration.ClientClaude] = "claude"
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{
				Enabled:    true,
				Executable: executableFixture(t, "claude"),
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "never-read-this-token"); err != nil {
				t.Fatal(err)
			}
			diagnosticStore, err := secrets.ForKind(secretStore, secrets.ProviderDiagnostic)
			if err != nil {
				t.Fatal(err)
			}
			if err := account.NewBackendStore(diagnosticStore, secrets.IsNotFound).Set("team", account.Credential{SystemToken: "never-read-this-system-token", UserID: "never-read-this-user"}); err != nil {
				t.Fatal(err)
			}
			before := readFile(t, app.Config.Path())
			beforeFiles := directoryNames(t, filepath.Dir(app.Config.Path()))

			tokenObservation := &recordingSecretStore{Store: secretStore}
			diagnosticObservation := &recordingSecretStore{Store: diagnosticStore}
			app.Secrets = tokenObservation
			app.Accounts = account.NewBackendStore(diagnosticObservation, secrets.IsNotFound)
			prompt := &recordingPrompt{}
			app.Interactive = true
			app.Prompt = prompt

			if err := execute(t, app, command); err != nil {
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
			if prompt.calls != 0 {
				t.Fatalf("%s prompted %d times", command, prompt.calls)
			}
			if len(runner.runPlans) != 0 {
				t.Fatalf("%s started a client process: %#v", command, runner.runPlans)
			}
			if calls := app.HTTP.(*fakeHTTP).calls; calls != 0 {
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
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(
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
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{
		Enabled:    true,
		Executable: executableFixture(t, "claude"),
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "never-read-this-token"); err != nil {
		t.Fatal(err)
	}
	diagnosticStore, err := secrets.ForKind(secretStore, secrets.ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential metadata unavailable")
	observed := &recordingSecretStore{Store: observationFailureStore{Store: diagnosticStore, err: want}}
	app.Accounts = account.NewBackendStore(observed, secrets.IsNotFound)

	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	if len(observed.getCalls) != 0 {
		t.Fatalf("status read diagnostic credential values %q", observed.getCalls)
	}
	if !strings.Contains(out.String(), "Credential metadata unavailable") || strings.Count(out.String(), "aigw doctor") != 1 || strings.Contains(out.String(), "aigw check") {
		t.Fatalf("status did not present one safe metadata recovery action:\n%s", out.String())
	}
}

func canonicalClientState(document canonicalReadinessDocument, client string) (string, string) {
	state, ok := document.Clients[client]
	if !ok {
		return "", ""
	}
	return state.State, state.NextAction
}
