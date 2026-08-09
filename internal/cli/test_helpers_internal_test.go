package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"aigw-cli/internal/account"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

type staticDiscovery struct{ result discovery.Result }

func (candidate staticDiscovery) Discover() discovery.Result { return candidate.result }

type commandRunner struct{ err error }

func (runner commandRunner) Run(context.Context, process.Plan) error { return runner.err }

func configuredCommandState() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "http://127.0.0.1:1234/v1", Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Purpose: "Primary", Account: "one", Models: configuration.Models{configuration.ClientCodex: "gpt", configuration.ClientClaude: "claude"}}
	cfg.Routes.Default = "one"
	return cfg
}

func configuredCommandApp(t *testing.T, cfg configuration.Config) *App {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return &App{Config: store, ClaudeSettingsPath: filepath.Join(t.TempDir(), ".claude", "settings.json"), Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore(), Discovery: staticDiscovery{}, Out: out, Err: out}
}
