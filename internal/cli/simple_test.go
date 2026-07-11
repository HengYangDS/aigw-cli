package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func twoProfileConfig() domain.Config {
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One Gateway", domain.Endpoints{Anthropic: "https://one.test", OpenAIResponses: "https://one.test/v1"}, "", domain.Models{})
	addAccountProfile(&cfg, "two", "two", "Two Gateway", domain.Endpoints{Anthropic: "https://two.test", OpenAIResponses: "https://two.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "one"
	return cfg
}

func TestUseWithoutNameSelectsProfileAndCollectsMissingToken(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	if err := app.Config.Save(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-token")
	prompt := &fakePrompt{selected: "two", secret: "two-token"}
	app.Interactive = true
	app.Prompt = prompt
	if err := execute(t, app, "use"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := app.Config.Load()
	if cfg.Routes.Default != "two" || !secretStore.Has("two") || prompt.secretCalls != 1 {
		t.Fatalf("config=%#v hasSecret=%v prompts=%d", cfg, secretStore.Has("two"), prompt.secretCalls)
	}
}

func TestRotateWithoutNameUsesCurrentProfileAndOnePaste(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := twoProfileConfig()
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-token")
	prompt := &fakePrompt{secret: "new-token"}
	app.Interactive = true
	app.Prompt = prompt
	if err := execute(t, app, "rotate"); err != nil {
		t.Fatal(err)
	}
	got, _ := secretStore.Get("one")
	if got != "new-token" || prompt.secretCalls != 1 {
		t.Fatalf("token=%q prompts=%d", got, prompt.secretCalls)
	}
}

func TestCheckProvidesOneClearHealthSummary(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "dmx"
	cfg.Adapters["claude"] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.Shims.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"配置文件", "系统密钥", "网关", "认证正常", "一切正常"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check lacks %q:\n%s", want, out.String())
		}
	}
}

func TestRepairDiscoversAndEnablesInstalledClients(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{ClaudeExecutable: "/opt/claude", CodexExecutable: "/opt/codex", CodexTargets: []string{target}}}
	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Adapters["claude"].Enabled || !got.Adapters["codex"].Enabled || len(runner.plans) != 1 {
		t.Fatalf("repair config=%#v plans=%#v", got, runner.plans)
	}
	if !strings.Contains(out.String(), "修复完成") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestHelpKeepsDailyCommandsObvious(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "--help"); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"use", "rotate", "check", "repair", "update"} {
		if !strings.Contains(out.String(), command) {
			t.Fatalf("help lacks %s:\n%s", command, out.String())
		}
	}
	for _, unwanted := range []string{"Usage:", "Additional Commands:", "Flags:"} {
		if strings.Contains(out.String(), unwanted) {
			t.Fatalf("help contains English scaffold %q:\n%s", unwanted, out.String())
		}
	}
	for _, wanted := range []string{"用法", "日常使用", "高级管理", "选项", "查看帮助", "显示版本"} {
		if !strings.Contains(out.String(), wanted) {
			t.Fatalf("help lacks Chinese section %q:\n%s", wanted, out.String())
		}
	}
}

func TestDoctorAcceptsOwnedClaudeShimWithoutPathDiscovery(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, "", domain.Models{})
	cfg.Routes.Default = "dmx"
	cfg.Adapters["claude"] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.Shims.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{}}
	err := execute(t, app, "doctor")
	if err != nil || !strings.Contains(out.String(), "shim:claude") || !strings.Contains(out.String(), "AIGW managed") {
		t.Fatalf("doctor did not accept the owned shim; err=%v output=%s", err, out.String())
	}
}

func TestRepairRestoresClaudeShimWithoutReplacingConfiguredExecutable(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", domain.Endpoints{Anthropic: "https://example.test"}, "", domain.Models{})
	cfg.Routes.Default = "claude"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	app.Discovery = fakeDiscovery{result: discovery.Result{ClaudeExecutable: "/different/claude"}}

	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.Adapters[domain.ClientClaude].Executable; got != "/opt/claude-real" {
		t.Fatalf("repair replaced configured Claude executable: %q", got)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); err != nil {
		t.Fatalf("repair did not restore owned Claude shim: %v", err)
	}
	if !strings.Contains(out.String(), "未改动") {
		t.Fatalf("repair incorrectly claimed authentication refresh:\n%s", out.String())
	}
}

func TestRepairCanRestoreClaudeWithoutAnyCodexProfile(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", domain.Endpoints{Anthropic: "https://example.test"}, domain.ClientClaude, domain.Models{Claude: "claude-test"})
	cfg.Routes.Default = "claude"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	app.Discovery = fakeDiscovery{result: discovery.Result{}}

	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	ready, err := app.Shims.ClaudeShimReady()
	if err != nil || !ready {
		t.Fatalf("Claude shim readiness = %v, %v", ready, err)
	}
}

func TestStatusWarnsWhenClaudePathActivationIsMissing(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", domain.Endpoints{Anthropic: "https://example.test"}, domain.ClientClaude, domain.Models{Claude: "claude-test"})
	cfg.Routes.Default = "claude"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	shimDir := filepath.Join(home, "Library", "Application Support", "aigw", "bin")
	app.Shims.BinDir = shimDir
	app.Shims.Home = home
	app.Shims.Shell = "/bin/zsh"
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if err := os.MkdirAll(shimDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shimDir, "claude"), []byte("#!/bin/sh\n# AIGW managed Claude shim\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude PATH 激活缺失") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("status did not surface missing Claude PATH activation:\n%s", out.String())
	}
}

func TestRotateAccountNamePromptsWithAccountLabel(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = domain.Profile{Label: "GPT Profile", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "old-token")
	prompt := &fakePrompt{secret: "new-token"}
	app.Interactive = true
	app.Prompt = prompt
	if err := execute(t, app, "rotate", "dmx"); err != nil {
		t.Fatal(err)
	}
	if prompt.lastSecretLabel != "请粘贴 DMXAPI Token：" {
		t.Fatalf("prompt label = %q", prompt.lastSecretLabel)
	}
}

func TestStatusGuidesClientSpecificRouteInsteadOfBlankRepair(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{Claude: "claude-fable-5"}}
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "Claude             ·") || strings.Contains(text, "aigw repair") {
		t.Fatalf("status should not show blank Claude route or misleading repair:\n%s", text)
	}
	for _, want := range []string{"Claude", "未选择 Claude Profile", "aigw use claude-fable-5 --for claude"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status lacks %q:\n%s", want, text)
		}
	}
}

func TestStatusWarnsWhenEnabledClaudeAdapterHasNoOwnedShim(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude Fable", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{Claude: "claude-fable-5"}}
	cfg.Routes.Default = "claude-fable-5"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
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
	if !strings.Contains(text, "Claude shim 缺失") || !strings.Contains(text, "aigw repair") {
		t.Fatalf("status did not surface the missing Claude shim:\n%s", text)
	}
}

func TestCheckFailsWhenEnabledClaudeAdapterHasNoOwnedShim(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude Fable", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{Claude: "claude-fable-5"}}
	cfg.Routes.Default = "claude-fable-5"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "check")
	if err == nil || !strings.Contains(out.String(), "Claude shim") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("check did not block on a missing Claude shim; err=%v output=%s", err, out.String())
	}
}

func TestCheckSuggestsAccountSpecificBalanceCommand(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "aigw account connect dmx") || !strings.Contains(text, "aigw balance dmx") {
		t.Fatalf("check should suggest account-specific diagnostics:\n%s", text)
	}
}

func TestStatusSuggestsAccountSpecificDiagnostics(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "aigw account connect dmx") {
		t.Fatalf("status should suggest account-specific diagnostics:\n%s", text)
	}
}

func TestCheckKeepsGenericHealthAvailableWhenExactDiagnosticDriverIsNotBundled(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["future"] = domain.Account{
		Label:        "Future Gateway",
		Endpoints:    domain.Endpoints{OpenAIResponses: "https://future.test/v1"},
		AccountProbe: &domain.AccountProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "future", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("future", "test-token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "当前版本未提供此服务商诊断") || strings.Contains(out.String(), "aigw balance") {
		t.Fatalf("check output = %s", out.String())
	}
}

func TestBalanceExplainsWhenConfiguredDiagnosticDriverIsNotBundled(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["future"] = domain.Account{
		Label:        "Future Gateway",
		Endpoints:    domain.Endpoints{OpenAIResponses: "https://future.test/v1"},
		AccountProbe: &domain.AccountProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "future", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("future", "test-token"); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "balance")
	if err == nil || !strings.Contains(err.Error(), "未包含在当前 AIGW 版本") || !strings.Contains(err.Error(), "aigw check") {
		t.Fatalf("balance error = %v", err)
	}
}
