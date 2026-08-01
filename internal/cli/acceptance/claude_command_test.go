package cli_test

import (
	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunClaudeSurfacesPreflightAndRunnerFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := claude.Run(context.Background(), app.Config, app.Secrets, app.Runner, nil, app.Env); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("adapter disabled", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		if err := claude.Run(context.Background(), app.Config, app.Secrets, app.Runner, nil, app.Env); err == nil || !strings.Contains(err.Error(), "not enabled") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route resolution", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/x"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		_ = secretStore.Set("one", "token")
		if err := claude.Run(context.Background(), app.Config, app.Secrets, app.Runner, nil, app.Env); err == nil || !strings.Contains(err.Error(), "is for codex") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/x"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := claude.Run(context.Background(), app.Config, app.Secrets, app.Runner, nil, app.Env); err == nil || !strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("runner", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/x"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		_ = secretStore.Set("one", "token")
		want := errors.New("runner failed")
		app.Runner = &failingRunner{err: want, remaining: 1}
		if err := claude.Run(context.Background(), app.Config, app.Secrets, app.Runner, nil, app.Env); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}
