package cli_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets/native"
)

func TestCheckDoesNotRecommendRotationForUnreadableToken(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	saveCommandRoute(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
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
