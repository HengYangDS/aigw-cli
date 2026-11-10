package cli_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
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
			// The parse stage and its position locate the failure without the
			// document, and the fix names the configuration's path, never a
			// command that would load it again.
			for _, want := range []string{"Cannot parse local configuration", "stopped at the configuration parse stage at line 1", "aigw config path"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s output lacks %q:\n%s", strings.Join(command, " "), want, text)
				}
			}
			for _, forbidden := range []string{"parse config:", "validate config:", "version = [", app.Config.Path(), "aigw check --depth local"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s leaked %q:\n%s", strings.Join(command, " "), forbidden, text)
				}
			}
		})
	}
}

type canonicalReadinessDocument struct {
	Clients clientStates `json:"clients"`
}

func TestExternalCredentialPolicyDoesNotRequireAnAIGWToken(t *testing.T) {
	app, out, _, runner, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://example.invalid"}}
	cfg.Models["claude"] = qualifiedModel("Claude", "gateway", "fixture", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, "claude", "")
	command := filepath.Join(t.TempDir(), "credential adapter")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	binding := cfg.Clients[claude.ID]
	binding.CredentialCommand = command
	cfg.Clients[claude.ID] = binding
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	store := &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), existsErr: errors.New("native metadata must not select external credentials")}
	app.Secrets = store
	for _, args := range [][]string{{"sync"}, {"check", "--json"}, {"status", "--json"}, {"check", "--depth", "local"}, {"route", "list"}, {"route", "show", "claude"}, {"use", "claude", "gateway/fixture"}, {"client", "disable", "claude"}, {"client", "enable", "claude", "--executable", cfg.Clients[claude.ID].Executable}} {
		out.Reset()
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v required a native Token: %v", args, err)
		}
		if args[0] == "status" || slices.Equal(args, []string{"check", "--json"}) {
			var result canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			state := result.Clients[claude.ID]
			if state.State != "configured" || state.NextAction != "aigw check claude --depth client" {
				t.Fatalf("external credential readiness = %#v", state)
			}
		}
		if slices.Equal(args, []string{"check", "--depth", "local"}) {
			after, err := app.Config.CaptureSnapshot()
			if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
				t.Fatalf("diagnosis changed external credential policy: %v", err)
			}
			if !strings.Contains(out.String(), "An external credential helper gives Claude Code its Token") || strings.Contains(out.String(), "rotate") || strings.Contains(out.String(), "secret:gateway") {
				t.Fatalf("diagnosis confused credential retrieval with wire authentication: %s", out)
			}
		}
	}
	if len(store.existsCalls)+len(store.getCalls)+len(store.setCalls)+len(store.deleteCalls) != 0 {
		t.Fatal("local admission observed or mutated external credentials")
	}
	if len(runner.plans) != 0 {
		t.Fatal("local admission executed a client or credential helper")
	}
	out.Reset()
	if err := cli.Execute(app, []string{"check", "claude", "--depth", "endpoint"}); err != nil || !strings.Contains(out.String(), "external credential helper") {
		t.Fatalf("the endpoint check did not explain external credential ownership: %v\n%s", err, out)
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
		{name: "endpoint checked", status: http.StatusOK, wantState: "configured"},
		{name: "invalid token", status: http.StatusUnauthorized, body: `{"message":"invalid token"}`, wantState: "invalid", wantError: true},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"slow down"}`, wantState: "degraded", wantError: true},
		{name: "unexpected response", status: http.StatusTeapot, body: `{"message":"unexpected"}`, wantState: "unavailable", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, _, httpClient := testApp(t, "")
			app.Version = "1.0.0"
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(
				&cfg,
				"claude",
				"team",
				"Team",
				domain.Endpoints{Anthropic: "https://team.test"},
				claude.ID,
				"claude-test",
			)
			cfg.SetSelectedModel(claude.ID, "claude", "")
			cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "token"); err != nil {
				t.Fatal(err)
			}
			httpClient.status = test.status
			httpClient.body = test.body

			err := cli.Execute(app, []string{"check", "--json", "--depth", "endpoint"})
			if (err != nil) != test.wantError {
				t.Fatalf("check error = %v, want error %v\n%s", err, test.wantError, out.String())
			}
			// The depth reached, not the state, says what a request proved.
			client, _ := decodeCheck(t, out).client(claude.ID)
			wantDepth := "endpoint"
			if test.wantError {
				wantDepth = "local"
			}
			if client.State != test.wantState || client.Depth != wantDepth {
				t.Fatalf("check state = %q at %q, want %q at %q\n%s", client.State, client.Depth, test.wantState, wantDepth, out.String())
			}
		})
	}
}

func TestReadinessDistinguishesConfigurationFromEndpointCheck(t *testing.T) {
	for _, command := range []string{"status"} {
		t.Run(command, func(t *testing.T) {
			app, out, secretStore, _, _ := testApp(t, "")
			app.Version = "1.0.0"
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(
				&cfg,
				"claude",
				"team",
				"Team",
				domain.Endpoints{Anthropic: "https://team.test"},
				claude.ID,
				"claude-test",
			)
			cfg.SetSelectedModel(claude.ID, "claude", "")
			cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
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
			state := document.Clients[claude.ID].State
			if state != "configured" {
				t.Fatalf("%s state = %q, want configured\n%s", command, state, out.String())
			}
		})
	}

	app, out, secretStore, _, _ := testApp(t, "")
	app.Version = "1.0.0"
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(
		&cfg,
		"claude",
		"team",
		"Team",
		domain.Endpoints{Anthropic: "https://team.test"},
		claude.ID,
		"claude-test",
	)
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"check", "--json", "--depth", "endpoint"}); err != nil {
		t.Fatalf("check failed: %v\n%s", err, out.String())
	}
	if client, _ := decodeCheck(t, out).client(claude.ID); client.State != "configured" || client.Depth != "endpoint" {
		t.Fatalf("check = %+v, want configured at the endpoint depth\n%s", client, out.String())
	}
}

func TestReadOnlyCommandsShareDeferredClientState(t *testing.T) {
	commands := []string{"status", "check"}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			app.Version = "1.0.0"
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(
				&cfg,
				"claude",
				"team",
				"Team",
				domain.Endpoints{Anthropic: "https://team.test"},
				claude.ID,
				"claude-test",
			)
			cfg.SetRecommendedModel(claude.ID, "claude")
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}

			if err := cli.Execute(app, []string{command, "--json"}); (err != nil) != (command == "check") {
				t.Fatalf("%s empty-scope outcome = %v\n%s", command, err, out.String())
			}
			if command == "check" {
				// Check names no client while none is enabled, so it checks
				// none and says that activation waits.
				report := decodeCheck(t, out)
				if report.OK || len(report.Clients) != 0 || report.State != "deferred" || report.Problem != "No client is enabled" {
					t.Fatalf("check without an enabled client = %+v", report)
				}
				return
			}
			var document canonicalReadinessDocument
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("decode %s JSON: %v\n%s", command, err, out.String())
			}
			client := document.Clients[claude.ID]
			state, action := client.State, client.NextAction
			if state != "deferred" || action != "aigw use claude team/claude-test" {
				t.Fatalf("%s state = %q, next_action = %q; want deferred and an explicit Claude selection\n%s", command, state, action, out.String())
			}
		})
	}
}

func TestStatusAndDoctorObserveCredentialMetadataWithoutSideEffects(t *testing.T) {
	for _, command := range []string{"status"} {
		t.Run(command, func(t *testing.T) {
			app, _, secretStore, runner, httpClient := testApp(t, "")
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(
				&cfg,
				"claude",
				"team",
				"Team",
				domain.Endpoints{Anthropic: "https://team.test"},
				claude.ID,
				"claude-test",
			)
			cfg.Providers["team"] = domain.Provider{
				Label:        "Team",
				Endpoints:    domain.Endpoints{Anthropic: "https://team.test"},
				BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"},
			}
			cfg.SetSelectedModel(claude.ID, "claude", "")
			cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "never-read-this-token"); err != nil {
				t.Fatal(err)
			}
			diagnosticCredentialStore := app.Providers
			if err := diagnosticCredentialStore.Set("team", secret.DiagnosticCredential{SystemToken: "never-read-this-system-token", UserID: "never-read-this-user"}); err != nil {
				t.Fatal(err)
			}
			before := readFile(t, app.Config.Path())
			beforeFiles := directoryNames(t, filepath.Dir(app.Config.Path()))

			tokenObservation := &recordingCredentialStore[string]{backend: secretStore}
			diagnosticObservation := &recordingCredentialStore[secret.DiagnosticCredential]{backend: diagnosticCredentialStore}
			app.Secrets = tokenObservation
			app.Providers = diagnosticObservation
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
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(
		&cfg,
		"claude",
		"team",
		"Team",
		domain.Endpoints{Anthropic: "https://team.test"},
		claude.ID,
		"claude-test",
	)
	cfg.Providers["team"] = domain.Provider{
		Label:        "Team",
		Endpoints:    domain.Endpoints{Anthropic: "https://team.test"},
		BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"},
	}
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "never-read-this-token"); err != nil {
		t.Fatal(err)
	}
	diagnosticCredentialStore, err := secret.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential metadata unavailable")
	observed := &recordingCredentialStore[secret.DiagnosticCredential]{backend: diagnosticCredentialStore, existsErr: want}
	app.Providers = observed

	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	if len(observed.getCalls) != 0 {
		t.Fatalf("status read diagnostic credential values %q", observed.getCalls)
	}
	if !strings.Contains(out.String(), "Balance credentials unknown") || strings.Count(out.String(), "aigw provider show team") != 1 || strings.Contains(out.String(), "aigw check --depth local") || strings.Contains(out.String(), "aigw check") {
		t.Fatalf("status did not present one safe metadata recovery action:\n%s", out.String())
	}
}

// Status reports Claude's projection ready only once its settings are
// synchronized, and check reads the Token and reaches the endpoint only then;
// neither reads the Token while observing readiness.
func TestClaudeReadinessFollowsSettingsConvergence(t *testing.T) {
	app, out, _, _, endpoint := testApp(t, "")
	app.Version = "1.0.0"
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{
		Label: "One",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://claude.example.test",
			OpenAIResponses: "https://codex.example.test/v1",
		},
	}
	cfg.Models["claude"] = domain.Model{
		Label: "Claude", Provider: "one", Family: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.Models["codex"] = domain.Model{
		Label: "Codex", Provider: "one", Family: "gpt-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetSelectedModel(codex.ID, "codex", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := &recordingCredentialStore[string]{backend: secret.NewMemoryStore()}
	if err := secretStore.backend.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	app.Secrets = secretStore
	for _, synchronized := range []bool{false, true} {
		if synchronized {
			synchronizeClaudeProjection(t, app, cfg)
		}
		out.Reset()
		secretReads := len(secretStore.getCalls)
		if err := cli.Execute(app, []string{"status", "--json"}); err != nil {
			t.Fatal(err)
		}
		var status struct {
			Clients map[string]struct {
				ProjectionReady bool `json:"client_files_ready"`
			} `json:"clients"`
		}
		if err := json.Unmarshal(out.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if client := status.Clients[claude.ID]; client.ProjectionReady != synchronized || len(secretStore.getCalls) != secretReads {
			t.Errorf("synchronized=%t client=%+v secret reads=%d", synchronized, client, len(secretStore.getCalls)-secretReads)
		}
		err := cli.Execute(app, []string{"check"})
		if synchronized {
			if err != nil || endpoint.calls == 0 {
				t.Errorf("synchronized check: error=%v requests=%d", err, endpoint.calls)
			}
		} else if err == nil || endpoint.calls != 0 || len(secretStore.getCalls) != secretReads || !strings.Contains(out.String(), "aigw sync") {
			t.Errorf("unsynchronized check: error=%v requests=%d secret reads=%d output=%s", err, endpoint.calls, len(secretStore.getCalls)-secretReads, out.String())
		}
	}
}
