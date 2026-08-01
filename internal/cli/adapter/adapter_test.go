package adapter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

type adapterDiscovery struct{ result discovery.Result }

func (d adapterDiscovery) Discover() discovery.Result { return d.result }

type adapterRunner struct {
	plans []process.Plan
	err   error
}

func (r *adapterRunner) Run(_ context.Context, plan process.Plan) error {
	r.plans = append(r.plans, plan)
	return r.err
}

func adapterConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{
		Label: "Gateway",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://gateway.test",
			OpenAIResponses: "https://gateway.test/v1",
		},
	}
	cfg.Profiles["default"] = configuration.Profile{
		Label:   "Default",
		Account: "gateway",
		Models: configuration.Models{
			configuration.ClientClaude: "claude-test",
			configuration.ClientCodex:  "codex-test",
		},
	}
	cfg.Routes.Default = "default"
	return cfg
}

func adapterRuntime(t *testing.T, cfg configuration.Config) (invocation.Context, *bytes.Buffer, *secrets.MemoryStore, *adapterRunner) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	secretStore := secrets.NewMemoryStore()
	runner := &adapterRunner{}
	return invocation.Context{
		Config:    store,
		Secrets:   secretStore,
		Out:       out,
		RenderOut: out,
		Runner:    runner,
		Discovery: adapterDiscovery{},
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

func TestListReportsEveryAdapterState(t *testing.T) {
	cfg := adapterConfig()
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	runtime, out, _, _ := adapterRuntime(t, cfg)

	if err := executeAdapter(t, runtime, "list"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Claude", "Enabled", "/opt/claude", "Codex", "Disabled"} {
		if !strings.Contains(text, want) {
			t.Fatalf("list output %q does not contain %q", text, want)
		}
	}
}

func TestDiscoverReportsFoundAndMissingExecutables(t *testing.T) {
	bin := t.TempDir()
	claudePath := filepath.Join(bin, configuration.ClientClaude)
	if err := os.WriteFile(claudePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	runtime, out, _, _ := adapterRuntime(t, adapterConfig())

	if err := executeAdapter(t, runtime, "discover"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, claudePath) || !strings.Contains(text, "Codex") || !strings.Contains(text, "Not found") {
		t.Fatalf("discover output = %q", text)
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
		{name: "unknown client", args: []string{"enable", "future", "--executable", "/opt/future"}, want: "Client must be"},
		{name: "missing executable", args: []string{"enable", configuration.ClientClaude}, want: "--executable is required"},
		{name: "codex target required", args: []string{"enable", configuration.ClientCodex, "--executable", "/opt/codex"}, want: "requires at least one --target"},
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
	runtime, _, _, _ := adapterRuntime(t, missingEndpoint)
	err := executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", "/opt/claude")
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("missing endpoint error = %v", err)
	}

	runtime, _, _, _ = adapterRuntime(t, adapterConfig())
	err = executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", "/opt/claude")
	if err == nil || !strings.Contains(err.Error(), "missing a token") {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestEnableClaudePersistsAdapterAndLauncher(t *testing.T) {
	cfg := adapterConfig()
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	runtime.ClaudeLauncher = claude.Launcher{GOOS: "windows", BinDir: bin, AIGWExecutable: `C:\Program Files\AIGW\aigw.exe`}

	if err := executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", `C:\Program Files\Claude\claude.exe`); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := got.Adapters[configuration.ClientClaude]
	if !adapter.Enabled || adapter.Executable != `C:\Program Files\Claude\claude.exe` {
		t.Fatalf("saved adapter = %#v", adapter)
	}
	if _, err := os.Stat(filepath.Join(bin, "claude.cmd")); err != nil {
		t.Fatalf("launcher was not created: %v", err)
	}
	if !strings.Contains(out.String(), "Client enabled") {
		t.Fatalf("output = %q", out.String())
	}

	err = executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", "/replacement")
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
		ID:         string(surfaceidentity.CodexCLIStandalone),
		Authority:  string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target,
		Present:    true,
	}}}}

	if err := executeAdapter(t, runtime, "enable", configuration.ClientCodex, "--executable", "/opt/codex", "--target", target); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := got.Adapters[configuration.ClientCodex]
	if !adapter.Enabled || adapter.Executable != "/opt/codex" || len(adapter.Targets) != 1 || adapter.Targets[0] != target {
		t.Fatalf("saved adapter = %#v", adapter)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != "/opt/codex" {
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

func TestEnableCodexRejectsUnsafeTargetsAndUnavailableDiscovery(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}

	runtime.Discovery = nil
	err := executeAdapter(t, runtime, "enable", configuration.ClientCodex, "--executable", "/opt/codex", "--target", "/tmp/configuration.toml")
	if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
		t.Fatalf("nil discovery error = %v", err)
	}

	executable := filepath.Join(t.TempDir(), "codex")
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(surfaceidentity.CodexCLIStandalone), Executable: executable}}}}
	err = executeAdapter(t, runtime, "enable", configuration.ClientCodex, "--executable", "/opt/codex", "--target", executable)
	if err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("executable target error = %v", err)
	}

	foreign := filepath.Join(t.TempDir(), "configuration.toml")
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: "foreign-surface", ConfigPath: foreign}}}}
	err = executeAdapter(t, runtime, "enable", configuration.ClientCodex, "--executable", "/opt/codex", "--target", foreign)
	if err == nil || !strings.Contains(err.Error(), "foreign-surface") {
		t.Fatalf("foreign target error = %v", err)
	}
}

func TestEnableClaudeRollsBackLauncherWhenCommitFails(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.WriteFile(bin, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.ClaudeLauncher = claude.Launcher{GOOS: "windows", BinDir: bin, AIGWExecutable: `C:\aigw.exe`}

	err := executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", `C:\claude.exe`)
	if err == nil {
		t.Fatal("expected launcher creation error")
	}
	got, loadErr := runtime.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if got.Adapters[configuration.ClientClaude].Enabled {
		t.Fatal("configuration changed despite launcher creation failure")
	}
}

func TestAuthValidatesClientAndBindsCodex(t *testing.T) {
	cfg := adapterConfig()
	runtime, _, _, _ := adapterRuntime(t, cfg)

	err := executeAdapter(t, runtime, "auth", configuration.ClientClaude)
	if err == nil || !strings.Contains(err.Error(), "only for codex") {
		t.Fatalf("wrong client error = %v", err)
	}
	err = executeAdapter(t, runtime, "auth", configuration.ClientCodex)
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("disabled error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "configuration.toml")
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	runtime, out, secretStore, runner := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "auth", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || !strings.Contains(out.String(), "authentication bound") {
		t.Fatalf("plans = %#v, output = %q", runner.plans, out.String())
	}

	runner.err = os.ErrPermission
	err = executeAdapter(t, runtime, "auth", configuration.ClientCodex)
	if err == nil || !strings.Contains(err.Error(), "Failed to bind") {
		t.Fatalf("runner error = %v", err)
	}
}

func TestDisableHandlesUnknownAndAlreadyDisabledClients(t *testing.T) {
	runtime, out, _, _ := adapterRuntime(t, adapterConfig())
	err := executeAdapter(t, runtime, "disable", "future")
	if err == nil || !strings.Contains(err.Error(), "Client must be") {
		t.Fatalf("unknown client error = %v", err)
	}
	if err := executeAdapter(t, runtime, "disable", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Already disabled") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDisableClaudeRemovesOwnedLauncherAndAdapter(t *testing.T) {
	cfg := adapterConfig()
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: `C:\claude.exe`}
	runtime, out, _, _ := adapterRuntime(t, cfg)
	bin := filepath.Join(t.TempDir(), "bin")
	launcher := claude.Launcher{GOOS: "windows", BinDir: bin, AIGWExecutable: `C:\aigw.exe`}
	if _, err := launcher.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	runtime.ClaudeLauncher = launcher

	if err := executeAdapter(t, runtime, "disable", configuration.ClientClaude); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Adapters[configuration.ClientClaude]; ok {
		t.Fatalf("adapter was not deleted: %#v", got.Adapters)
	}
	if _, err := os.Stat(filepath.Join(bin, "claude.cmd")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("launcher remains: %v", err)
	}
	if !strings.Contains(out.String(), "Client disabled") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDisableClaudePreservesConfigWhenLauncherIsUnowned(t *testing.T) {
	cfg := adapterConfig()
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: `C:\claude.exe`}
	runtime, _, _, _ := adapterRuntime(t, cfg)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude.cmd"), []byte("@echo off\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runtime.ClaudeLauncher = claude.Launcher{GOOS: "windows", BinDir: bin, AIGWExecutable: `C:\aigw.exe`}

	err := executeAdapter(t, runtime, "disable", configuration.ClientClaude)
	if err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("unowned launcher error = %v", err)
	}
	got, loadErr := runtime.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !got.Adapters[configuration.ClientClaude].Enabled {
		t.Fatal("config was changed despite launcher refusal")
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
		ID: string(surfaceidentity.CodexCLIStandalone), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target,
	}}}}
	if err := executeAdapter(t, runtime, "enable", configuration.ClientCodex, "--executable", "/opt/codex", "--target", target); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "disable", configuration.ClientCodex); err != nil {
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

func TestCodexTargetValidationAcceptsStandaloneAndUnknownPaths(t *testing.T) {
	standalone := filepath.Join(t.TempDir(), "configuration.toml")
	discovered := discovery.Result{Surfaces: []discovery.Surface{{
		ID: string(surfaceidentity.CodexCLIStandalone), ConfigPath: standalone,
	}}}
	if err := ValidateCodexTarget(discovered, standalone); err != nil {
		t.Fatalf("standalone target rejected: %v", err)
	}
	if err := ValidateCodexTarget(discovered, filepath.Join(t.TempDir(), "explicit.toml")); err != nil {
		t.Fatalf("explicit target rejected: %v", err)
	}
}

func TestCommandsPropagateConfigurationLoadErrors(t *testing.T) {
	badPath := t.TempDir()
	runtime := invocation.Context{
		Config:    configuration.NewStore(badPath),
		Secrets:   secrets.NewMemoryStore(),
		Out:       &bytes.Buffer{},
		RenderOut: &bytes.Buffer{},
		Discovery: adapterDiscovery{},
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"list"}},
		{name: "enable", args: []string{"enable", configuration.ClientClaude, "--executable", "/opt/claude"}},
		{name: "auth", args: []string{"auth", configuration.ClientCodex}},
		{name: "disable", args: []string{"disable", configuration.ClientCodex}},
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
	profile := cfg.Profiles["default"]
	profile.Client = configuration.ClientCodex
	delete(profile.Models, configuration.ClientClaude)
	cfg.Profiles["default"] = profile
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	err := executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", "/opt/claude")
	if err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("runtime error = %v", err)
	}
}

func TestDisableCodexReturnsProjectionFailure(t *testing.T) {
	cfg := adapterConfig()
	target := filepath.Join(t.TempDir(), "missing", "configuration.toml")
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	runtime, _, _, _ := adapterRuntime(t, cfg)
	runtime.Discovery = adapterDiscovery{}

	err := executeAdapter(t, runtime, "disable", configuration.ClientCodex)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("disable error = %v", err)
	}
	got, loadErr := runtime.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !got.Adapters[configuration.ClientCodex].Enabled {
		t.Fatal("configuration was not rolled back")
	}
}

func TestRendererFallsBackToPrimaryOutput(t *testing.T) {
	out := &bytes.Buffer{}
	r := renderer(invocation.Context{Out: out})
	r.Title("AIGW", "Adapter")
	if !strings.Contains(out.String(), "Adapter") {
		t.Fatalf("output = %q", out.String())
	}
}
