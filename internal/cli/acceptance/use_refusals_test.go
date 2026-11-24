package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestUseSurfacesCredentialObservationFailure(t *testing.T) {
	t.Parallel()
	app, _, _, _, _ := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), existsErr: want}

	if err := cli.Execute(app, []string{"use", "claude", "one/claude-test"}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestUseSurfacesConfigLoadFailure(t *testing.T) {
	t.Parallel()
	app, _, _, _, _ := testApp(t, "")
	app.Config = store.NewStore(t.TempDir(), clients.Admission())
	err := cli.Execute(app, []string{"use", "claude", "one/claude-test"})
	if err == nil {
		t.Fatal("expected a config load failure")
	}
}

func TestUseRejectsUnknownModel(t *testing.T) {
	t.Parallel()
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-one"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	// A model that is not added is added once the provider's catalog lists
	// it; a catalog that cannot be read refuses it before any write, naming
	// --unlisted, which adds an ID that the provider serves unlisted.
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	err = cli.Execute(app, []string{"use", "claude", "one/does-not-exist"})
	if problem := terminal.ProblemFor(err); err == nil || !strings.Contains(err.Error(), "the one catalog cannot be read") ||
		problem.Fix != "aigw use claude one/does-not-exist --unlisted" {
		t.Fatalf("error = %v, fix %q", err, problem.Fix)
	}
	if after, err := app.Config.CaptureSnapshot(); err != nil || !after.Config.Equal(before.Config) {
		t.Fatalf("a refused addition wrote the configuration: %v", err)
	}
}

func TestUseSurfacesUnknownProviderReference(t *testing.T) {
	t.Parallel()
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Models[domain.NewModelRef("one", "claude-one")] = qualifiedModel("One", "one", "claude-one", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-one"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	// Store.Save validates referential integrity, so a selection of a
	// provider that the file does not hold can only come from a file edited
	// outside AIGW, by hand or by another tool, after the fact.
	data, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n[clients.codex]\nenabled = false\nmodel = \"ghost/claude-broken\"\n")...)
	if err := os.WriteFile(app.Config.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	// Loading the configuration refuses the selection, before any Token is
	// read.
	err = cli.Execute(app, []string{"use", "claude", "ghost/claude-broken"})
	if err == nil || !strings.Contains(err.Error(), "names the unknown model ghost/claude-broken") {
		t.Fatalf("error = %v", err)
	}
}
