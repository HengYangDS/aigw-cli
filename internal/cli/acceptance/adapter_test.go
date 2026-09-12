package cli_test

import (
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
	if err == nil || !strings.Contains(err.Error(), "claude or codex") {
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
