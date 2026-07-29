package cli_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestProfileShowRendersEverySecretFreeField(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["shared"] = domain.Account{Label: "Shared", Endpoints: domain.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Profiles["both"] = domain.Profile{Label: "Both Models", Purpose: "Daily work", Account: "shared", Models: domain.Models{domain.ClientCodex: "gpt-test", domain.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "both"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("shared", "never-render-this"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "profile", "show", "both"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Both Models", "Daily work", "shared", "gpt-test", "claude-test", "https://shared.test/v1", "https://shared.test", "Available"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("human output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("human output leaked token: %s", out.String())
	}
	out.Reset()
	if err := execute(t, app, "profile", "show", "both", "--json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"secret_available":true`) || strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("JSON output = %s", out.String())
	}
}

func TestAdvancedProfileAndAccountValidationBranches(t *testing.T) {
	t.Run("profile add invalid id", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		err := execute(t, app, "profile", "add", "bad id", "--account", "one", "--for", "claude", "--model", "m")
		if err == nil || !strings.Contains(err.Error(), "Invalid profile ID") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "add", "two", "--account", "one", "--for", "claude", "--model", "m"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("profile add duplicate", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "profile", "add", "one", "--account", "one", "--for", "claude", "--model", "m")
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add unknown account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "profile", "add", "two", "--account", "missing", "--for", "claude", "--model", "m")
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add default label", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		if err := execute(t, app, "profile", "add", "two", "--account", "one", "--for", "claude", "--model", "m2"); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Profiles["two"].Label != "two" {
			t.Fatalf("profile = %#v", cfg.Profiles["two"])
		}
	})

	t.Run("account edit requires change", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "account", "edit", "one"); err == nil {
			t.Fatal("expected nothing-to-update error")
		}
	})

	t.Run("account edit load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "account", "edit", "one", "--label", "New"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("account edit unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "account", "edit", "missing", "--label", "New")
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("account edit label and anthropic", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		if err := execute(t, app, "account", "edit", "one", "--label", "Renamed", "--anthropic-url", "https://new.test/"); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Accounts["one"].Label != "Renamed" || cfg.Accounts["one"].Endpoints.Anthropic != "https://new.test" {
			t.Fatalf("account = %#v", cfg.Accounts["one"])
		}
	})
}

func TestAdvancedProfileReadEditAndRemoveErrors(t *testing.T) {
	t.Run("list load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "list"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "show", "one"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		err := execute(t, app, "profile", "show", "missing")
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("edit requires change", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "profile", "edit", "one"); err == nil {
			t.Fatal("expected nothing-to-update error")
		}
	})

	t.Run("edit load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "edit", "one", "--label", "New"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("edit unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "profile", "edit", "missing", "--label", "New")
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "remove", "one"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("remove unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "profile", "remove", "missing")
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove override", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		cfg := domain.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m1"})
		addAccountProfile(&cfg, "two", "one", "Two", domain.Endpoints{}, domain.ClientCodex, domain.Models{domain.ClientCodex: "m2"})
		cfg.Routes.Default = "one"
		cfg.Routes.Overrides[domain.ClientCodex] = "two"
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		err := execute(t, app, "profile", "remove", "two")
		if err == nil || !strings.Contains(err.Error(), "used by codex") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestProfileEditSynchronizesActiveCodexProjection(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
	cfg.Routes.Default = "one"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "token")
	if err := execute(t, app, "profile", "edit", "one", "--label", "New Label"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "configuration synchronized") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRouteResetValidationLoadAndSuccess(t *testing.T) {
	t.Run("invalid client", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "route", "reset", "other"); err == nil {
			t.Fatal("expected client validation error")
		}
	})

	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "route", "reset", "codex"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("success", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		cfg := domain.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		cfg.Routes.Default = "one"
		cfg.Routes.Overrides[domain.ClientCodex] = "one"
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "route", "reset", "codex"); err != nil {
			t.Fatal(err)
		}
		got, _ := app.Config.Load()
		if _, ok := got.Routes.Overrides[domain.ClientCodex]; ok || !strings.Contains(out.String(), "inherits") {
			t.Fatalf("routes=%#v output=%q", got.Routes, out.String())
		}
	})
}

func TestAdapterReadAndValidationBranches(t *testing.T) {
	t.Run("list load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "adapter", "list"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("list enabled executable", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
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
		if err := execute(t, app, "adapter", "discover"); err != nil {
			t.Fatal(err)
		}
		for name, path := range resolved {
			if !strings.Contains(out.String(), path) {
				t.Fatalf("output missing resolved %s path %q: %q", name, path, out.String())
			}
		}
	})

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

	t.Run("enable already enabled", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/old"}
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
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		err := execute(t, app, "adapter", "enable", "claude", "--executable", "/x")
		if err == nil || !strings.Contains(err.Error(), "is for codex") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable missing token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		err := execute(t, app, "adapter", "enable", "claude", "--executable", "/x")
		if err == nil || !strings.Contains(err.Error(), "missing a token") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("enable missing discovery", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		app.Discovery = nil
		err := execute(t, app, "adapter", "enable", "codex", "--executable", "/x", "--target", filepath.Join(t.TempDir(), "config.toml"))
		if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("auth bind failure", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/x", Targets: []string{filepath.Join(t.TempDir(), "config.toml")}}
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

func TestConfigCommandIOFailures(t *testing.T) {
	t.Run("path output", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		want := errors.New("output failed")
		app.Out = failingOutput{err: want}
		if err := execute(t, app, "config", "path"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("export load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "config", "export"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("export output", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		want := errors.New("output failed")
		app.Out = failingOutput{err: want}
		if err := execute(t, app, "config", "export"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("import read", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "config", "import", filepath.Join(t.TempDir(), "missing.toml")); err == nil {
			t.Fatal("expected manifest read failure")
		}
	})

	t.Run("import parse", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		path := filepath.Join(t.TempDir(), "bad.toml")
		if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "config", "import", path); err == nil {
			t.Fatal("expected manifest parse failure")
		}
	})
}

func TestRunClaudeSurfacesPreflightAndRunnerFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := cli.RunClaude(app, nil); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("adapter disabled", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		if err := cli.RunClaude(app, nil); err == nil || !strings.Contains(err.Error(), "not enabled") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route resolution", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/x"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		_ = secretStore.Set("one", "token")
		if err := cli.RunClaude(app, nil); err == nil || !strings.Contains(err.Error(), "is for codex") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/x"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := cli.RunClaude(app, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("runner", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		cfg, _ := app.Config.Load()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/x"}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		_ = secretStore.Set("one", "token")
		want := errors.New("runner failed")
		app.Runner = &failingRunner{err: want, remaining: 1}
		if err := cli.RunClaude(app, nil); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}
