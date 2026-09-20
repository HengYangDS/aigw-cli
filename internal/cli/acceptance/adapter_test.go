package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

func TestAdapterEnableSurfacesCredentialObservationFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: want}

	err := cli.Execute(app, []string{"adapter", "enable", "claude", "--executable", executableFixture(t, "claude")})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestAdapterEnableReportsConfigurationCommitFailure(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	if err := secretStore.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	backupPath := app.Config.Path() + ".bak"
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.Mkdir(backupPath, 0o700); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"adapter", "enable", "claude", "--executable", executableFixture(t, "claude")})
	if err == nil || !strings.Contains(err.Error(), "Adapter enablement failed and was rolled back") {
		t.Fatalf("error = %v", err)
	}
}

func TestAdapterListAndDiscoveryBranches(t *testing.T) {
	t.Run("list load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"adapter", "list"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("list enabled executable", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := cli.Execute(app, []string{"adapter", "list"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Enabled") || !strings.Contains(out.String(), "/opt/claude") {
			t.Fatalf("output = %q", out.String())
		}
	})

	t.Run("discover executables", func(t *testing.T) {
		dir := t.TempDir()
		fixtures := map[string]string{}
		for _, name := range []string{"claude", "codex"} {
			filename := name
			content := []byte("#!/bin/sh\nexit 0\n")
			mode := os.FileMode(0o700)
			if runtime.GOOS == "windows" {
				filename = name + ".cmd"
				content = []byte("@echo off\r\nexit /b 0\r\n")
				mode = 0o600
			}
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, content, mode); err != nil {
				t.Fatal(err)
			}
			fixtures[name] = path
		}
		t.Setenv("PATH", dir)
		resolved := map[string]string{}
		for name, fixture := range fixtures {
			path, err := exec.LookPath(name)
			if err != nil {
				t.Fatalf("resolve %s fixture: %v", name, err)
			}
			assertSameExistingPath(t, path, fixture)
			resolved[name] = path
		}
		app, out, _, _, _ := testApp(t, "")
		app.Discovery = client.NewDiscoverer(client.DefaultRegistry(), discovery.System{GOOS: runtime.GOOS, Home: t.TempDir(), Path: dir})
		if err := cli.Execute(app, []string{"adapter", "discover"}); err != nil {
			t.Fatal(err)
		}
		for name, path := range resolved {
			if !strings.Contains(out.String(), path) {
				t.Fatalf("output missing resolved %s path %q: %q", name, path, out.String())
			}
		}
	})
}

func TestAdapterValidationBranches(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "enable invalid", args: []string{"adapter", "enable", "other", "--executable", "/x"}, want: "invalid argument"},
		{name: "enable missing executable", args: []string{"adapter", "enable", "claude"}, want: "--executable is required"},
		{name: "enable missing target", args: []string{"adapter", "enable", "codex", "--executable", "/x"}, want: "requires at least one"},
		{name: "disable invalid", args: []string{"adapter", "disable", "other"}, want: "invalid argument"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			err := cli.Execute(app, test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAdapterStateFailureBranches(t *testing.T) {
	t.Run("enable already enabled", func(t *testing.T) {
		app, _, secretStore, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/old"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		_ = secretStore.Set("one", "token")
		err := cli.Execute(app, []string{"adapter", "enable", "claude", "--executable", "/new"})
		if err == nil || !strings.Contains(err.Error(), "already enabled") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable unresolved route", func(t *testing.T) {
		app, _, secretStore, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		_ = secretStore.Set("one", "token")
		err := cli.Execute(app, []string{"adapter", "enable", "claude", "--executable", "/x"})
		if err == nil || !strings.Contains(err.Error(), "no route selected for client \"claude\"") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable missing token", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"adapter", "enable", "claude", "--executable", "/x"})
		if err == nil || !strings.Contains(err.Error(), "missing a token") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable missing discovery", func(t *testing.T) {
		app, _, secretStore, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		_ = secretStore.Set("one", "token")
		app.Discovery = nil
		err := cli.Execute(app, []string{"adapter", "enable", "codex", "--executable", "/x", "--target", filepath.Join(t.TempDir(), "configuration.toml")})
		if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("disable already disabled", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"adapter", "disable", "codex"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Already disabled") {
			t.Fatalf("output = %q", out.String())
		}
	})
}

func TestAdapterEnableClaudeStoresOnlyClaudeExecutable(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-model")
	cfg.Routes[configuration.ClientClaude] = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := cli.Execute(app, []string{"adapter", "enable", "claude", "--executable", claudeExecutable}); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Adapters["claude"].Enabled || got.Adapters["claude"].Executable != claudeExecutable {
		t.Fatalf("Claude adapter = %#v", got.Adapters["claude"])
	}
	if _, exists := got.Adapters["codex"]; exists {
		t.Fatalf("Claude enable touched Codex: %#v", got.Adapters)
	}
	if err := cli.Execute(app, []string{"adapter", "disable", "claude"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claudeExecutable); err != nil {
		t.Fatalf("adapter disable changed the foreign Claude executable: %v", err)
	}
}

func TestAdapterCommandsListOnlyAdmittedClients(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}, configuration.ClientClaude, "claude-model")
	cfg.Routes[configuration.ClientClaude] = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"adapter", "list"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Claude", "Codex"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("adapter list misses admitted client %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(strings.ToLower(out.String()), "gemini") || strings.Contains(strings.ToLower(out.String()), "qwen") {
		t.Fatalf("adapter list exposed an unadmitted client:\n%s", out.String())
	}

	err := cli.Execute(app, []string{"profile", "add", "future", "--account", "team", "--for", "gemini", "--model", "gemini-next"})
	if err == nil || !strings.Contains(err.Error(), "claude, codex, or hermes") {
		t.Fatalf("unadmitted client error = %v", err)
	}
}

func TestAdapterEnableAndDisableCodexOwnsOnlyConfiguredTarget(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "codex", "configuration.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "model_provider = \"native\"\nmodel = \"gpt-test\"\n"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}, configuration.ClientCodex, "gpt-model")
	cfg.Routes[configuration.ClientCodex] = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := cli.Execute(app, []string{"adapter", "enable", "codex", "--executable", "/opt/codex-real", "--target", target}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("projection invoked a native client: %#v", runner.plans)
	}
	projected, _ := os.ReadFile(target)
	if !strings.Contains(string(projected), "AIGW managed provider") {
		t.Fatalf("target not projected:\n%s", projected)
	}
	if err := cli.Execute(app, []string{"adapter", "disable", "codex"}); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(target)
	if string(restored) != original {
		t.Fatalf("target not restored:\n%s", restored)
	}
}

func TestSyncPreservesExplicitCredentialCommandsAcrossAIGWUpgrade(t *testing.T) {
	app, _, store, runner, _ := testApp(t, "")
	root := t.TempDir()
	target := filepath.Join(root, "config.toml")
	writeFile(t, target, []byte("model_provider = \"native\"\nforeign = true\n"), 0o600)
	writeFile(t, app.ClaudeSettingsPath, []byte(`{"theme":"dark","env":{"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS":"1"}}`), 0o600)
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	for _, id := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		executable := filepath.Join(root, id)
		writeFile(t, executable, []byte("public fixture"), 0o700)
		cfg.Profiles[id] = configuration.Profile{Label: id, Account: "gateway", Client: id, Model: "fixture-model"}
		cfg.Routes[id] = id
		cfg.Adapters[id] = configuration.AdapterConfig{Enabled: true, Executable: executable}
	}
	adapter := cfg.Adapters[configuration.ClientCodex]
	adapter.Targets = []string{target}
	cfg.Adapters[configuration.ClientCodex] = adapter
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("gateway", "public-fixture-token"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := cli.Execute(app, []string{"sync"}); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(root, "credential adapter")
	for _, id := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		adapter := cfg.Adapters[id]
		adapter.CredentialCommand = command
		cfg.Adapters[id] = adapter
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	beforeCodex := readFile(t, target)
	beforeClaude := readFile(t, app.ClaudeSettingsPath)
	beforeConfig := readFile(t, app.Config.Path())
	if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeCodex, readFile(t, target)) || !bytes.Equal(beforeClaude, readFile(t, app.ClaudeSettingsPath)) || !bytes.Equal(beforeConfig, readFile(t, app.Config.Path())) {
		t.Fatal("dry-run wrote configuration")
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	projectedCodex := readFile(t, target)
	projectedClaude := readFile(t, app.ClaudeSettingsPath)
	if !strings.Contains(string(projectedCodex), "credential adapter") || !strings.Contains(string(projectedClaude), "credential adapter") {
		t.Fatal("native projection omitted explicit command")
	}
	app.Executable = filepath.Join(root, "upgraded-aigw")
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(projectedCodex, readFile(t, target)) || !bytes.Equal(projectedClaude, readFile(t, app.ClaudeSettingsPath)) {
		t.Fatal("AIGW executable update replaced explicit helper")
	}
	assertCredentialPolicyDisableReenable(t, app, root, target, command)
	if len(runner.plans) != 0 {
		t.Fatal("sync started a native client or credential reader")
	}
	if token, err := store.Get("gateway"); err != nil || token != "public-fixture-token" {
		t.Fatal("sync changed credential")
	}
}

func assertCredentialPolicyDisableReenable(t *testing.T, app *cli.App, root, target, command string) {
	t.Helper()
	for _, id := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		if err := cli.Execute(app, []string{"adapter", "disable", id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	disabled, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		adapter := disabled.Adapters[id]
		if adapter.Enabled || adapter.CredentialCommand != command {
			t.Fatalf("sync did not preserve disabled %s policy: %#v", id, adapter)
		}
		args := []string{"adapter", "enable", id, "--executable", filepath.Join(root, id)}
		if id == configuration.ClientCodex {
			args = append(args, "--target", target)
		}
		if err := cli.Execute(app, args); err != nil {
			t.Fatal(err)
		}
	}
	reenabled, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		if adapter := reenabled.Adapters[id]; !adapter.Enabled || adapter.CredentialCommand != command {
			t.Fatalf("reenable replaced %s credential policy: %#v", id, adapter)
		}
	}
}

func TestExplicitClaudeVerificationUsesSynchronizedHelperWithoutNativeToken(t *testing.T) {
	app, out, _, runner, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://example.invalid"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "fixture-model"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	command := filepath.Join(t.TempDir(), "explicit-helper")
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude"), CredentialCommand: command}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	beforeConfig := readFile(t, app.Config.Path())
	beforeSettings := readFile(t, app.ClaudeSettingsPath)
	app.Secrets = nil
	if err := cli.Execute(app, []string{"verify", "--for", "claude"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != cfg.Adapters[configuration.ClientClaude].Executable {
		t.Fatal("verification did not use the sole native client plan")
	}
	if !bytes.Equal(beforeConfig, readFile(t, app.Config.Path())) || !bytes.Equal(beforeSettings, readFile(t, app.ClaudeSettingsPath)) {
		t.Fatal("verification mutated configuration")
	}
	out.Reset()
	runner.output = []byte("public-secret-canary")
	runner.capture = errors.New("public-secret-canary")
	err := cli.Execute(app, []string{"verify", "--for", "claude"})
	if err == nil || strings.Contains(err.Error(), "public-secret-canary") || strings.Contains(out.String(), "public-secret-canary") {
		t.Fatal("unknown helper diagnostics escaped or failure was accepted")
	}
	if len(runner.plans) != 2 {
		t.Fatal("verification retried or fell back")
	}
	runner.capture = nil
	if err := cli.Execute(app, []string{"verify", "--for", "claude"}); err == nil || strings.Contains(err.Error(), "public-secret-canary") {
		t.Fatal("non-marker client success was accepted or disclosed its output")
	}
}
