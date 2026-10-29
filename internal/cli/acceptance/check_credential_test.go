package cli_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/native"
)

func TestCheckDoesNotRecommendRotationForUnreadableToken(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("one", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	want := fmt.Errorf("private credential detail: %w", native.ErrUnavailable)
	app.Secrets = &recordingCredentialStore[string]{backend: secretStore, getErr: want}

	err = cli.Execute(app, []string{"check", "--for", "claude"})
	if !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("check error = %v, want unreadable credential", err)
	}
	for _, text := range []string{"Claude account token could not be read", "aigw doctor"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("check output lacks %q: %s", text, out.String())
		}
	}
	for _, forbidden := range []string{"aigw rotate", "private credential detail", "fixture-token"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("check output contains %q: %s", forbidden, out.String())
		}
	}
	if httpClient.calls != 0 {
		t.Fatalf("unreadable credential sent %d provider requests", httpClient.calls)
	}
}

// The environment backend cannot rotate a Token, so a rejected Token names the
// variable that supplies it, in human and JSON output alike.
func TestCheckNamesTheEnvironmentTokenForARejectedToken(t *testing.T) {
	app, out, _, _, httpClient := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, claude.ID, "claude-test")
	cfg.SetSelectedRoute(claude.ID, "dmx", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	variable := secrets.EnvironmentKey("dmx")
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
		if key == variable {
			return "rejected-fixture-token"
		}
		return ""
	})
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"message":"invalid token"}`)), Request: req}, nil
	}
	for _, args := range [][]string{{"check", "--endpoint-only"}, {"check", "--endpoint-only", "--json"}} {
		out.Reset()
		_ = cli.Execute(app, args)
		if !strings.Contains(out.String(), variable) || strings.Contains(out.String(), "aigw rotate") || strings.Contains(out.String(), "rejected-fixture-token") {
			t.Fatalf("%v recovery does not follow the environment backend:\n%s", args, out.String())
		}
	}
}
