package cli_test

import (
	"encoding/json"
	"net/http"
	"testing"

	configuration "aigw-cli/internal/configuration"
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

func canonicalClientState(document canonicalReadinessDocument, client string) (string, string) {
	state, ok := document.Clients[client]
	if !ok {
		return "", ""
	}
	return state.State, state.NextAction
}
