package cli_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

func TestSetupRejectsInvalidFlagCombinationsAndValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty from", args: []string{"setup", "--from="}, want: "--from requires"},
		{name: "missing profile", args: []string{"setup"}, want: "--profile is required"},
		{name: "invalid account", args: []string{"setup", "--profile", "one", "--account", "bad id"}, want: "Invalid account ID"},
		{name: "invalid profile", args: []string{"setup", "--profile", "bad id", "--account", "one"}, want: "Invalid profile ID"},
		{name: "model without client", args: []string{"setup", "--profile", "one", "--model", "m"}, want: "--model requires"},
		{name: "claude endpoint", args: []string{"setup", "--profile", "one", "--for", "claude", "--model", "m"}, want: "requires --anthropic-url"},
		{name: "claude model", args: []string{"setup", "--profile", "one", "--for", "claude", "--anthropic-url", "https://one.test"}, want: "requires --model"},
		{name: "codex endpoint", args: []string{"setup", "--profile", "one", "--for", "codex", "--model", "m"}, want: "requires --openai-url"},
		{name: "codex model", args: []string{"setup", "--profile", "one", "--for", "codex", "--openai-url", "https://one.test/v1"}, want: "requires --model"},
		{name: "unknown client", args: []string{"setup", "--profile", "one", "--for", "other"}, want: "--for must be"},
		{name: "invalid empty endpoints", args: []string{"setup", "--profile", "one"}, want: "at least one endpoint"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _ := testApp(t, "")
			err := execute(t, app, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestSetupSurfacesStateAndDependencyFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "token\n")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("already configured", func(t *testing.T) {
		app, _, _, _ := testApp(t, "token\n")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "setup", "--profile", "two", "--for", "claude", "--model", "m", "--anthropic-url", "https://two.test", "--token-stdin")
		if err == nil || !strings.Contains(err.Error(), "already configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("interactive load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "setup"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("interactive already configured", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "setup")
		if err == nil || !strings.Contains(err.Error(), "already configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret lookup", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		want := errors.New("keychain unavailable")
		app.Secrets = &failingSecretsStore{getErr: want}
		err := execute(t, app, "setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test")
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("token validation", func(t *testing.T) {
		app, _, _, _ := testApp(t, "token\n")
		app.HTTP.(*fakeHTTP).status = http.StatusUnauthorized
		err := execute(t, app, "setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin")
		if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, _, _, _ := testApp(t, "token\n")
		app.Discovery = nil
		err := execute(t, app, "setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin")
		if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret write", func(t *testing.T) {
		app, _, _, _ := testApp(t, "token\n")
		want := errors.New("keychain locked")
		app.Secrets = &failingSecretsStore{getErr: secrets.ErrNotFound, setErr: want}
		err := execute(t, app, "setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin")
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestSetupRollsBackSecretWhenDiscoveredClientEnablementFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "token\n")
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Shims.BinDir = filepath.Join(blocker, "bin")
	app.Shims.AIGWExecutable = "/bin/aigw"
	app.Discovery = fakeDiscovery{result: discovery.Result{ClaudeExecutable: "/opt/claude"}}
	err := execute(t, app, "setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin")
	if err == nil {
		t.Fatal("expected Claude launcher enablement failure")
	}
	if secretStore.Has("one") {
		t.Fatal("failed launcher enablement left the new token")
	}
}

func TestSetupRollsBackConfigAndSecretWhenCodexProjectionFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "token\n")
	target := t.TempDir()
	app.Discovery = fakeDiscovery{result: discovery.Result{
		CodexExecutable: "/opt/codex",
		Surfaces:        []discovery.Surface{{ID: discovery.SurfaceCodexCLIStandalone, Authority: discovery.AuthorityAIGW, ConfigPath: target, Present: true, AutoManaged: true}},
	}}
	err := execute(t, app, "setup", "--profile", "one", "--for", "codex", "--model", "m", "--openai-url", "https://one.test/v1", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("one") {
		t.Fatal("failed setup left the new token")
	}
}

func TestTeamSetupSurfacesManifestAndConfigFailures(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "setup", "--from", filepath.Join(t.TempDir(), "missing.toml")); err == nil {
			t.Fatal("expected manifest read failure")
		}
	})

	t.Run("parse", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		path := writeTeamSetupManifest(t, "not = [valid")
		if err := execute(t, app, "setup", "--from", path); err == nil {
			t.Fatal("expected manifest parse failure")
		}
	})

	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		path := writeTeamSetupManifest(t, teamSetupManifest)
		if err := execute(t, app, "setup", "--from", path); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("already configured", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		path := writeTeamSetupManifest(t, teamSetupManifest)
		err := execute(t, app, "setup", "--from", path)
		if err == nil || !strings.Contains(err.Error(), "already configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Discovery = nil
		path := writeTeamSetupManifest(t, teamSetupManifest)
		err := execute(t, app, "setup", "--from", path)
		if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
}
