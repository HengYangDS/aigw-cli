package client

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	surfaceidentity "aigw-cli/internal/adapter/surface"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

	"github.com/pelletier/go-toml/v2"
)

type adapterDiscovery struct{ result discovery.Result }

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: goruntime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

// claudeSettingsPath returns the Claude Code settings file on host.
func claudeSettingsPath(t testing.TB, host discovery.Host) string {
	t.Helper()
	path, err := claude.SettingsPath(host)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func (d adapterDiscovery) Discover() discovery.Result { return d.result }

type adapterRunner struct {
	plans []process.Plan
	err   error
}

func (r *adapterRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	r.plans = append(r.plans, plan)
	return nil, r.err
}

func (r *adapterRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := r.RunCapture(ctx, plan)
	return output, nil, err
}

func adapterConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{
		Label: "Gateway",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://gateway.test",
			OpenAIResponses: "https://gateway.test/v1",
		},
	}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "gateway", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["codex"] = domain.Route{Label: "Codex", Account: "gateway", Model: "codex-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	return cfg
}

func adapterRuntime(t *testing.T, cfg domain.Config) (invocation.Context, *bytes.Buffer, secret.Store, *adapterRunner) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	secretStore := secret.NewMemoryStore()
	runner := &adapterRunner{}
	return invocation.Context{Clients: clients.Registry(),
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Config:     configStore,
		Host:       hostAt(t.TempDir()),
		Secrets:    secretStore,
		Out:        out,
		RenderOut:  out,
		Runner:     runner,
		Discovery:  adapterDiscovery{},
	}, out, secretStore, runner
}

func executeAdapter(t *testing.T, runtime invocation.Context, args ...string) error {
	t.Helper()
	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(args)
	return command.Execute()
}

func TestDiscoverReportsFoundAndMissingExecutables(t *testing.T) {
	bin := t.TempDir()
	claudePath := filepath.Join(bin, claude.ID)
	if err := os.WriteFile(claudePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runtime, out, _, _ := adapterRuntime(t, adapterConfig())
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: claudePath}}}

	if err := executeAdapter(t, runtime, "discover"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, claudePath) || !strings.Contains(text, "Codex") || !strings.Contains(text, "Not found") {
		t.Fatalf("discover output = %q", text)
	}
}

func TestDiscoverFailsWhenDiscoveryIsUnavailable(t *testing.T) {
	runtime, _, _, _ := adapterRuntime(t, adapterConfig())
	runtime.Discovery = nil
	if err := executeAdapter(t, runtime, "discover"); err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
		t.Fatalf("discover error = %v", err)
	}
}

func TestEnableRejectsInvalidInputsBeforeMutation(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown client", args: []string{"enable", "future", "--executable", "/opt/future"}, want: "invalid argument"},
		{name: "missing executable", args: []string{"enable", claude.ID}, want: "--executable is required"},
		{name: "codex target required", args: []string{"enable", codex.ID, "--executable", "/opt/codex"}, want: "requires at least one --target"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := executeAdapter(t, runtime, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEnableRejectsInvalidConfigurationAndMissingSecret(t *testing.T) {
	missingEndpoint := adapterConfig()
	account := missingEndpoint.Accounts["gateway"]
	account.Endpoints.Anthropic = ""
	missingEndpoint.Accounts["gateway"] = account
	runtime, _, _, _ := adapterRuntime(t, adapterConfig())
	data, err := toml.Marshal(missingEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtime.Config.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	err = executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/opt/claude")
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("missing endpoint error = %v", err)
	}

	runtime, _, _, _ = adapterRuntime(t, adapterConfig())
	err = executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/opt/claude")
	if err == nil || !strings.Contains(err.Error(), "missing a token") {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestEnableClaudePersistsOnlyTheRealExecutable(t *testing.T) {
	cfg := adapterConfig()
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "enable", claude.ID, "--executable", `C:\Program Files\Claude\claude.exe`); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := got.Clients[claude.ID]
	if !adapter.Enabled || adapter.Executable != `C:\Program Files\Claude\claude.exe` {
		t.Fatalf("saved adapter = %#v", adapter)
	}
	if !strings.Contains(out.String(), "Client enabled") {
		t.Fatalf("output = %q", out.String())
	}

	err = executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/replacement")
	if err == nil || !strings.Contains(err.Error(), "already enabled") {
		t.Fatalf("duplicate enable error = %v", err)
	}
}

func TestEnableCodexValidatesTargetsAndPersistsProjection(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, runner := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model = \"original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
		ID:         string(codex.HomeDefault),
		Authority:  string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target,
		Present:    true,
	}}}}

	if err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", target); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := got.Clients[codex.ID]
	if !adapter.Enabled || adapter.Executable != "/opt/codex" || len(adapter.Targets) != 1 || adapter.Targets[0] != target {
		t.Fatalf("saved adapter = %#v", adapter)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("authentication plans = %#v", runner.plans)
	}
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projected), "AIGW managed provider") {
		t.Fatalf("target was not projected: %s", projected)
	}
}

func TestEnableCodexWithClientNativeAuthenticationDoesNotAccessAccountTokens(t *testing.T) {
	cfg := adapterConfig()
	binding := cfg.Clients[codex.ID]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = domain.AuthenticationClientNative
	cfg.Clients[codex.ID] = binding
	runtime, _, _, runner := adapterRuntime(t, cfg)
	runtime.Secrets = unavailableAdapterSecretStore{}
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
		ID:         string(codex.HomeDefault),
		Authority:  string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target,
		Present:    true,
	}}}}

	if err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", target); err != nil {
		t.Fatalf("enable client-native Codex: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("client-native enable attempted credential binding: %#v", runner.plans)
	}
}

func TestEnableCodexRejectsUnsafeTargetsAndUnavailableDiscovery(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}

	runtime.Discovery = nil
	err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", "/tmp/configuration.toml")
	if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
		t.Fatalf("nil discovery error = %v", err)
	}

	executable := filepath.Join(t.TempDir(), "codex")
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(codex.HomeDefault), Executable: executable}}}}
	err = executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", executable)
	if err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("executable target error = %v", err)
	}

	foreign := filepath.Join(t.TempDir(), "configuration.toml")
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: "foreign-surface", ConfigPath: foreign}}}}
	err = executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", foreign)
	if err == nil || !strings.Contains(err.Error(), "foreign-surface") {
		t.Fatalf("foreign target error = %v", err)
	}
}

func TestEnableClaudeDoesNotRequireAnAIGWLauncherDirectory(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	err := executeAdapter(t, runtime, "enable", claude.ID, "--executable", `C:\claude.exe`)
	if err != nil {
		t.Fatal(err)
	}
	got, loadErr := runtime.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !got.Clients[claude.ID].Enabled {
		t.Fatal("Claude adapter was not enabled")
	}
}

type unavailableAdapterSecretStore struct{}

func (unavailableAdapterSecretStore) Get(string) (string, error) {
	return "", errors.New("secret store must not be accessed")
}

func (unavailableAdapterSecretStore) Set(string, string) error {
	return errors.New("secret store must not be accessed")
}

func (unavailableAdapterSecretStore) Delete(string) error {
	return errors.New("secret store must not be accessed")
}

func (unavailableAdapterSecretStore) Exists(string) (bool, error) {
	return false, errors.New("secret store must not be accessed")
}

func TestDisableHandlesUnknownAndAlreadyDisabledClients(t *testing.T) {
	runtime, out, _, _ := adapterRuntime(t, adapterConfig())
	err := executeAdapter(t, runtime, "disable", "future")
	if err == nil || !strings.Contains(err.Error(), "invalid argument") {
		t.Fatalf("unknown client error = %v", err)
	}
	if err := executeAdapter(t, runtime, "disable", codex.ID); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Already disabled") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDisableClaudeRemovesOnlyTheAIGWAdapter(t *testing.T) {
	cfg := adapterConfig()
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := os.MkdirAll(filepath.Dir(claudeSettingsPath(t, runtime.Host)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeSettingsPath(t, runtime.Host), []byte("{\n  \"theme\": \"dark\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "enable", claude.ID, "--executable", `C:\claude.exe`); err != nil {
		t.Fatal(err)
	}
	verified, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Config.SaveVerifiedCheckpoint(t.Context(), verified, []string{claude.ID}); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "disable", claude.ID); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if adapter, ok := got.Clients[claude.ID]; !ok || adapter.Enabled {
		t.Fatalf("explicit disabled intent was not retained: %#v", got.Clients)
	}
	if got.SelectedRoute(claude.ID) != "claude" || got.Routes["claude"].Account != "gateway" {
		t.Fatalf("capability configuration changed: %#v", got)
	}
	if token, err := secretStore.Get("gateway"); err != nil || token != "token" {
		t.Fatalf("credential = %q, %v; want preserved", token, err)
	}
	settings, err := os.ReadFile(claudeSettingsPath(t, runtime.Host))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), `"theme": "dark"`) || strings.Contains(string(settings), "apiKeyHelper") || strings.Contains(string(settings), "ANTHROPIC_BASE_URL") {
		t.Fatalf("restored Claude settings = %s", settings)
	}
	for _, path := range []string{claudeSettingsPath(t, runtime.Host) + ".aigw-state.json", runtime.Config.Path() + ".verified.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("retired AIGW state remains at %s: %v", path, err)
		}
	}
	backup, err := runtime.Config.LoadBackup()
	if err != nil || !backup.Clients[claude.ID].Enabled {
		t.Fatalf("explicit previous configuration = %#v, %v", backup, err)
	}
	if !strings.Contains(out.String(), "Client disabled") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDisableClaudeDoesNotInspectForeignFilesystemEntries(t *testing.T) {
	cfg := adapterConfig()
	cfg.SetClientActivation(claude.ID, true, `C:\claude.exe`, nil)
	runtime, _, _, _ := adapterRuntime(t, cfg)
	err := executeAdapter(t, runtime, "disable", claude.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, loadErr := runtime.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if got.Clients[claude.ID].Enabled {
		t.Fatal("Claude adapter remains enabled")
	}
}

func TestDisableCodexRestoresProjection(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model = \"original\"\n"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
		ID: string(codex.HomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target,
	}}}}
	if err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", target); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "disable", codex.ID); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("restored target = %q", data)
	}
}

func TestCommandsPropagateConfigurationLoadErrors(t *testing.T) {
	badPath := t.TempDir()
	runtime := invocation.Context{Clients: clients.Registry(),
		Config:    store.NewStore(badPath, clients.Admission()),
		Secrets:   secret.NewMemoryStore(),
		Out:       &bytes.Buffer{},
		RenderOut: &bytes.Buffer{},
		Discovery: adapterDiscovery{},
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"list"}},
		{name: "enable", args: []string{"enable", claude.ID, "--executable", "/opt/claude"}},
		{name: "auth", args: []string{"auth", codex.ID}},
		{name: "disable", args: []string{"disable", codex.ID}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := executeAdapter(t, runtime, test.args...); err == nil {
				t.Fatal("expected configuration load error")
			}
		})
	}
}

func TestEnablePropagatesRuntimeResolutionError(t *testing.T) {
	cfg := adapterConfig()
	delete(cfg.Clients, claude.ID)
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	err := executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/opt/claude")
	if err == nil || !strings.Contains(err.Error(), "no Route selected") {
		t.Fatalf("runtime error = %v", err)
	}
}

func TestDisableCodexReturnsProjectionFailure(t *testing.T) {
	cfg := adapterConfig()
	target := filepath.Join(t.TempDir(), "missing", "configuration.toml")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	runtime, _, _, _ := adapterRuntime(t, cfg)
	runtime.Discovery = adapterDiscovery{}

	err := executeAdapter(t, runtime, "disable", codex.ID)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("disable error = %v", err)
	}
	got, loadErr := runtime.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !got.Clients[codex.ID].Enabled {
		t.Fatal("configuration was not rolled back")
	}
}

func TestDiscoverFallsBackToPrimaryOutput(t *testing.T) {
	runtime, out, _, _ := adapterRuntime(t, adapterConfig())
	runtime.RenderOut = nil
	if err := executeAdapter(t, runtime, "discover"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Client discovery") {
		t.Fatalf("output = %q", out.String())
	}
}
