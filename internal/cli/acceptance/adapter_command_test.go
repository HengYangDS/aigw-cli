package cli_test

import (
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAdapterEnableSurfacesCredentialObservationFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	want := errors.New("credential observation failed")
	app.Secrets = observationFailureStore{Store: secrets.NewMemoryStore(), err: want}

	err := execute(t, app, "adapter", "enable", "claude", "--executable", executableFixture(t, "claude"))
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestAdapterEnableReportsConfigurationCommitFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
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

	err := execute(t, app, "adapter", "enable", "claude", "--executable", executableFixture(t, "claude"))
	if err == nil || !strings.Contains(err.Error(), "Adapter enablement failed and was rolled back") {
		t.Fatalf("error = %v", err)
	}
}

func TestAdapterListAndDiscoveryBranches(t *testing.T) {
	t.Run("list load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "adapter", "list"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("list enabled executable", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "adapter", "list"); err != nil {
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
		app, out, _, _ := testApp(t, "")
		app.Discovery = client.NewDiscoverer(client.DefaultRegistry(), discovery.System{GOOS: runtime.GOOS, Home: t.TempDir(), Path: dir})
		if err := execute(t, app, "adapter", "discover"); err != nil {
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
		{name: "enable invalid", args: []string{"adapter", "enable", "other", "--executable", "/x"}, want: "must be claude or codex"},
		{name: "enable missing executable", args: []string{"adapter", "enable", "claude"}, want: "--executable is required"},
		{name: "enable missing target", args: []string{"adapter", "enable", "codex", "--executable", "/x"}, want: "requires at least one"},
		{name: "auth invalid", args: []string{"adapter", "auth", "claude"}, want: "only for codex"},
		{name: "auth disabled", args: []string{"adapter", "auth", "codex"}, want: "not enabled"},
		{name: "disable invalid", args: []string{"adapter", "disable", "other"}, want: "must be claude or codex"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _ := testApp(t, "")
			err := execute(t, app, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAdapterStateFailureBranches(t *testing.T) {
	t.Run("enable already enabled", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/old"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		_ = secretStore.Set("one", "token")
		err := execute(t, app, "adapter", "enable", "claude", "--executable", "/new")
		if err == nil || !strings.Contains(err.Error(), "already enabled") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable unresolved route", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		_ = secretStore.Set("one", "token")
		err := execute(t, app, "adapter", "enable", "claude", "--executable", "/x")
		if err == nil || !strings.Contains(err.Error(), "no route selected for client \"claude\"") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable missing token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := execute(t, app, "adapter", "enable", "claude", "--executable", "/x")
		if err == nil || !strings.Contains(err.Error(), "missing a token") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable missing discovery", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		_ = secretStore.Set("one", "token")
		app.Discovery = nil
		err := execute(t, app, "adapter", "enable", "codex", "--executable", "/x", "--target", filepath.Join(t.TempDir(), "configuration.toml"))
		if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("auth bind failure", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		cfg, _ := app.Config.Load()
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/x", Targets: []string{filepath.Join(t.TempDir(), "configuration.toml")}}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		err := execute(t, app, "adapter", "auth", "codex")
		if err == nil || !strings.Contains(err.Error(), "Failed to bind") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("disable already disabled", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		if err := execute(t, app, "adapter", "disable", "codex"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Already disabled") {
			t.Fatalf("output = %q", out.String())
		}
	})
}

func TestAdapterEnableClaudeStoresOnlyClaudeExecutable(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-model")
	cfg.Routes[configuration.ClientClaude] = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := execute(t, app, "adapter", "enable", "claude", "--executable", claudeExecutable); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Adapters["claude"].Enabled || got.Adapters["claude"].Executable != claudeExecutable {
		t.Fatalf("Claude adapter = %#v", got.Adapters["claude"])
	}
	if _, exists := got.Adapters["codex"]; exists {
		t.Fatalf("Claude enable touched Codex: %#v", got.Adapters)
	}
	if err := execute(t, app, "adapter", "disable", "claude"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claudeExecutable); err != nil {
		t.Fatalf("adapter disable changed the foreign Claude executable: %v", err)
	}
}

func TestAdapterCommandsListOnlyAdmittedClients(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}, configuration.ClientClaude, "claude-model")
	cfg.Routes[configuration.ClientClaude] = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "adapter", "list"); err != nil {
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

	err := execute(t, app, "profile", "add", "future", "--account", "team", "--for", "gemini", "--model", "gemini-next")
	if err == nil || !strings.Contains(err.Error(), "claude or codex") {
		t.Fatalf("unadmitted client error = %v", err)
	}
}

func TestAdapterEnableAndDisableCodexOwnsOnlyConfiguredTarget(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
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
	if err := execute(t, app, "adapter", "enable", "codex", "--executable", "/opt/codex-real", "--target", target); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Stdin != "secret\n" {
		t.Fatalf("official login plan not run safely: %#v", runner.plans)
	}
	projected, _ := os.ReadFile(target)
	if !strings.Contains(string(projected), "AIGW managed provider") {
		t.Fatalf("target not projected:\n%s", projected)
	}
	if err := execute(t, app, "adapter", "disable", "codex"); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(target)
	if string(restored) != original {
		t.Fatalf("target not restored:\n%s", restored)
	}
}

func TestSyncHumanPreviewHandlesDisabledAndEnabledAdapters(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		if err := execute(t, app, "sync", "--dry-run"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "No client configuration needs changing") {
			t.Fatalf("output = %q", out.String())
		}
	})

	t.Run("enabled", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		target := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := configuration.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		cfg.Routes[configuration.ClientCodex] = "one"
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "sync", "--dry-run"); err != nil {
			t.Fatal(err)
		}
		var renderedTargets []string
		for _, line := range strings.Split(out.String(), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, " initial-project") {
				renderedTargets = append(renderedTargets, strings.TrimSpace(strings.TrimSuffix(line, " initial-project")))
			}
		}
		if len(renderedTargets) != 1 {
			t.Fatalf("initial-project rows = %#v, output = %q", renderedTargets, out.String())
		}
		assertSameExistingPath(t, renderedTargets[0], target)
	})
}

func TestStatusWarnsWhenEnabledClaudeAdapterExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude Fable", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-fable-5"}
	cfg.Routes[configuration.ClientClaude] = "claude-fable-5"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Claude executable is unavailable") || !strings.Contains(text, "aigw repair") {
		t.Fatalf("status did not surface the unavailable Claude executable:\n%s", text)
	}
}

func TestCheckFailsWhenEnabledClaudeAdapterExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude Fable", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-fable-5"}
	cfg.Routes[configuration.ClientClaude] = "claude-fable-5"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "check")
	if err == nil || !strings.Contains(out.String(), "Claude executable is unavailable") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("check did not block on an unavailable Claude executable; err=%v output=%s", err, out.String())
	}
}
