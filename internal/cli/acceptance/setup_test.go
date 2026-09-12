package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSetupReusesReadOnlyEnvironmentSecretWithoutPromptingOrPersisting(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("dmx") {
			return "environment-only-token"
		}
		return ""
	})

	if err := cli.Execute(app, []string{"setup",
		"--account", "dmx",
		"--profile", "gpt-5.6-terra",
		"--label", "DMXAPI",
		"--openai-url", "https://example.test/v1",
		"--for", "codex",
		"--model", "gpt-5.6-terra"},
	); err != nil {
		t.Fatalf("setup with existing environment secret: %v", err)
	}
	if !secretExists(t, app.Secrets, "dmx") {
		t.Fatal("environment secret was not retained as the active credential")
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes[configuration.ClientCodex] != "gpt-5.6-terra" || cfg.Profiles["gpt-5.6-terra"].Account != "dmx" {
		t.Fatalf("setup config = %#v", cfg)
	}
	if strings.Contains(out.String(), "environment-only-token") {
		t.Fatalf("environment secret leaked in setup output: %s", out.String())
	}
}

func TestSetupExplicitTokenStdinReplacesAnExistingStoredToken(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "stdin-token\n")
	app.Secrets = secretStore
	if err := secretStore.Set("dmx", "old-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"setup",
		"--account", "dmx",
		"--profile", "gpt-5.6-terra",
		"--label", "DMXAPI",
		"--openai-url", "https://example.test/v1",
		"--for", "codex",
		"--model", "gpt-5.6-terra",
		"--token-stdin"},
	); err != nil {
		t.Fatalf("setup with explicit stdin token: %v", err)
	}
	got, err := secretStore.Get("dmx")
	if err != nil || got != "stdin-token" {
		t.Fatalf("stored token = %q, %v; want explicit stdin token", got, err)
	}
}

func TestSetupRejectsInvalidFlagCombinationsAndValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty from", args: []string{"setup", "--from="}, want: "--from requires"},
		{name: "json without manifest", args: []string{"setup", "--json"}, want: "--json requires --from"},
		{name: "missing profile", args: []string{"setup"}, want: "--profile is required"},
		{name: "invalid account", args: []string{"setup", "--profile", "one", "--account", "bad id"}, want: "Invalid account ID"},
		{name: "invalid profile", args: []string{"setup", "--profile", "bad id", "--account", "one"}, want: "Invalid profile ID"},
		{name: "model without client", args: []string{"setup", "--profile", "one", "--model", "m"}, want: "--for is required"},
		{name: "claude endpoint", args: []string{"setup", "--profile", "one", "--for", "claude", "--model", "m"}, want: "requires --anthropic-url"},
		{name: "claude model", args: []string{"setup", "--profile", "one", "--for", "claude", "--anthropic-url", "https://one.test"}, want: "requires --model"},
		{name: "codex endpoint", args: []string{"setup", "--profile", "one", "--for", "codex", "--model", "m"}, want: "requires --openai-url"},
		{name: "codex model", args: []string{"setup", "--profile", "one", "--for", "codex", "--openai-url", "https://one.test/v1"}, want: "requires --model"},
		{name: "unknown client", args: []string{"setup", "--profile", "one", "--for", "other"}, want: "--for must be"},
		{name: "invalid unused endpoint", args: []string{"setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--openai-url", "http://remote.test/v1"}, want: "plain HTTP is allowed only"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			err := cli.Execute(app, test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestSetupSurfacesStateAndDependencyFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "token\n")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("already configured", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "token\n")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"setup", "--profile", "two", "--for", "claude", "--model", "m", "--anthropic-url", "https://two.test", "--token-stdin"})
		if err == nil || !strings.Contains(err.Error(), "already configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("interactive load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"setup"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("interactive already configured", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"setup"})
		if err == nil || !strings.Contains(err.Error(), "already configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret lookup", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		want := errors.New("keychain unavailable")
		app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), getErr: want}
		err := cli.Execute(app, []string{"setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test"})
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("token validation", func(t *testing.T) {
		app, _, _, _, httpClient := testApp(t, "token\n")
		httpClient.status = http.StatusUnauthorized
		err := cli.Execute(app, []string{"setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin"})
		if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "token\n")
		app.Discovery = nil
		err := cli.Execute(app, []string{"setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin"})
		if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret write", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "token\n")
		want := errors.New("keychain locked")
		app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), setErr: want}
		err := cli.Execute(app, []string{"setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin"})
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestSetupRejectsInvalidAutomaticBackendSelectionBeforeCredentialInput(t *testing.T) {
	tests := []struct {
		name string
		args func(*testing.T) []string
	}{
		{
			name: "guided setup",
			args: func(*testing.T) []string {
				return []string{"setup", "--profile", "one", "--for", "claude", "--model", "m", "--anthropic-url", "https://one.test", "--token-stdin"}
			},
		},
		{
			name: "manifest setup",
			args: func(t *testing.T) []string {
				return []string{"setup", "--from", writeConfigurationManifest(t, configurationManifestFixture), "--account", "dmxapi", "--token-stdin"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			app.In = failingReadCloser{err: errors.New("credential input was read")}
			secretsRoot := filepath.Join(t.TempDir(), "secrets")
			if err := os.MkdirAll(secretsRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(secretsRoot, "backend"), []byte("invalid\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := secrets.Select(secrets.Selection{GOOS: runtime.GOOS, Root: secretsRoot})
			if err != nil {
				t.Fatal(err)
			}
			app.Secrets = store

			err = cli.Execute(app, test.args(t))
			if err == nil || !strings.Contains(err.Error(), "invalid persisted secret backend") {
				t.Fatalf("error = %v", err)
			}
			assertSetupTransactionClosed(t, app.Config)
		})
	}
}

func TestSetupRollsBackConfigAndSecretWhenCodexProjectionFails(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "token\n")
	target := t.TempDir()
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/opt/codex"},
		Surfaces:    []discovery.Surface{{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target, Present: true, AutoManaged: true}},
	}}
	err := cli.Execute(app, []string{"setup", "--profile", "one", "--for", "codex", "--model", "m", "--openai-url", "https://one.test/v1", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "one") {
		t.Fatal("failed setup left the new token")
	}
	assertSetupTransactionClosed(t, app.Config, target+".aigw-state.json")
}

func TestSetupRestoresExistingSecretWhenCodexProjectionFails(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "new-token\n")
	if err := secretStore.Set("one", "old-token"); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/opt/codex"},
		Surfaces:    []discovery.Surface{{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target, Present: true, AutoManaged: true}},
	}}
	err := cli.Execute(app, []string{"setup", "--profile", "one", "--for", "codex", "--model", "m", "--openai-url", "https://one.test/v1", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if token, getErr := secretStore.Get("one"); getErr != nil || token != "old-token" {
		t.Fatalf("Token after rollback = %q, %v; want old-token", token, getErr)
	}
	assertSetupTransactionClosed(t, app.Config, target+".aigw-state.json")
}

func TestSetupRollbackRemovesNewAutomaticBackendSelection(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
	secretsRoot := filepath.Join(t.TempDir(), "secrets")
	store, err := secrets.Select(secrets.Selection{
		GOOS:         runtime.GOOS,
		Root:         secretsRoot,
		KeyringProbe: func(secrets.Store) error { return errors.New("native credential service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	app.Secrets = store
	target := t.TempDir()
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/opt/codex"},
		Surfaces:    []discovery.Surface{{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target, Present: true, AutoManaged: true}},
	}}

	err = cli.Execute(app, []string{"setup", "--profile", "one", "--for", "codex", "--model", "m", "--openai-url", "https://one.test/v1", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, store, "one") {
		t.Fatal("failed setup left the new token")
	}
	if _, err := os.Stat(filepath.Join(secretsRoot, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed setup left the automatic backend selection: %v", err)
	}
}

func assertSetupTransactionClosed(t *testing.T, store configuration.Store, absentPaths ...string) {
	t.Helper()
	configPath := store.Path()
	paths := append([]string{configPath, configPath + ".bak", configPath + ".verified.json"}, absentPaths...)
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed setup left owned state at %s: %v", path, err)
		}
	}
	roots := map[string]struct{}{filepath.Dir(configPath): {}}
	for _, path := range absentPaths {
		roots[filepath.Dir(path)] = struct{}{}
	}
	for root := range roots {
		pattern := filepath.Join(root, ".aigw-write-*")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("failed setup left temporary files: %#v", matches)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	unlock, err := store.Lock(ctx)
	if err != nil {
		t.Fatalf("failed setup retained the configuration lock: %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatalf("release configuration lock probe: %v", err)
	}
}
