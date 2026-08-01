package cli_test

import (
	"aigw-cli/internal/secrets"
	"strings"
	"testing"
)

func TestSetupReusesReadOnlyEnvironmentSecretWithoutPromptingOrPersisting(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("dmx") {
			return "environment-only-token"
		}
		return ""
	})

	if err := execute(t, app,
		"setup",
		"--account", "dmx",
		"--profile", "gpt-5.6-terra",
		"--label", "DMXAPI",
		"--openai-url", "https://example.test/v1",
		"--for", "codex",
		"--model", "gpt-5.6-terra",
	); err != nil {
		t.Fatalf("setup with existing environment secret: %v", err)
	}
	if !app.Secrets.Has("dmx") {
		t.Fatal("environment secret was not retained as the active credential")
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes.Default != "gpt-5.6-terra" || cfg.Profiles["gpt-5.6-terra"].Account != "dmx" {
		t.Fatalf("setup config = %#v", cfg)
	}
	if strings.Contains(out.String(), "environment-only-token") {
		t.Fatalf("environment secret leaked in setup output: %s", out.String())
	}
}

func TestSetupExplicitTokenStdinStillOverridesAnEnvironmentSecret(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "stdin-token\n")
	app.Secrets = secretStore
	if err := secretStore.Set("dmx", "old-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app,
		"setup",
		"--account", "dmx",
		"--profile", "gpt-5.6-terra",
		"--label", "DMXAPI",
		"--openai-url", "https://example.test/v1",
		"--for", "codex",
		"--model", "gpt-5.6-terra",
		"--token-stdin",
	); err != nil {
		t.Fatalf("setup with explicit stdin token: %v", err)
	}
	got, err := secretStore.Get("dmx")
	if err != nil || got != "stdin-token" {
		t.Fatalf("stored token = %q, %v; want explicit stdin token", got, err)
	}
}
