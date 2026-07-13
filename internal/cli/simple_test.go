package cli_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type fakeUpdater struct {
	updateCalls    int
	rollbackCalls  int
	updateResult   string
	rollbackResult string
	updateErr      error
	rollbackErr    error
}

func (u *fakeUpdater) Update(_ context.Context, _ string) (string, error) {
	u.updateCalls++
	return u.updateResult, u.updateErr
}

func (u *fakeUpdater) Rollback(_ context.Context) (string, error) {
	u.rollbackCalls++
	return u.rollbackResult, u.rollbackErr
}

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

func TestCheckProbesTheDefaultRouteInsteadOfAnOverride(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["claude-account"] = domain.Account{Label: "Claude Gateway", Endpoints: domain.Endpoints{Anthropic: "https://claude.test"}}
	cfg.Accounts["codex-account"] = domain.Account{Label: "Codex Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude Fable 5", Account: "claude-account", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-fable-5"}}
	cfg.Profiles["gpt-5.6-terra"] = domain.Profile{Label: "GPT-5.6 Terra", Account: "codex-account", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-terra"}}
	cfg.Routes.Default = "claude-fable-5"
	cfg.Routes.Overrides[domain.ClientCodex] = "gpt-5.6-terra"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "claude-token"); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("codex-account", "codex-token"); err != nil {
		t.Fatal(err)
	}
	var gotHost, gotAuthorization string
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		gotHost = req.URL.Host
		gotAuthorization = req.Header.Get("Authorization")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: req}, nil
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	if gotHost != "claude.test" || gotAuthorization != "Bearer claude-token" {
		t.Fatalf("check probe host=%q authorization=%q, want default Claude route", gotHost, gotAuthorization)
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

func TestRepairResyncsAnExistingTruncatedCodexProjection(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt-5.6-terra"})
	cfg.Routes.Default = "dmx"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(strings.Replace(string(projected), "# <<< AIGW managed provider <<<\n", "", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{CodexExecutable: "/opt/codex", CodexTargets: []string{target}}}

	if err := execute(t, app, "repair"); err != nil {
		t.Fatalf("repair did not resync the existing Codex projection: %v", err)
	}
	repaired, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repaired), "# <<< AIGW managed provider <<<\n") {
		t.Fatalf("repair falsely succeeded without restoring the provider terminator:\n%s", repaired)
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

func TestUpdateRollbackUsesLocalProgramRollbackOnly(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	updater := &fakeUpdater{rollbackResult: "已恢复上一程序版本；可再次运行 `aigw update --rollback` 恢复当前版本。"}
	app.Updater = updater
	if err := execute(t, app, "update", "--rollback"); err != nil {
		t.Fatal(err)
	}
	if updater.rollbackCalls != 1 || updater.updateCalls != 0 {
		t.Fatalf("update calls=%d rollback calls=%d", updater.updateCalls, updater.rollbackCalls)
	}
	if !strings.Contains(out.String(), "程序回退") || !strings.Contains(out.String(), "已恢复上一程序版本") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestUpdateWithoutRollbackKeepsNetworkUpdatePath(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{updateResult: "已更新到 v0.2.0。"}
	app.Updater = updater
	if err := execute(t, app, "update"); err != nil {
		t.Fatal(err)
	}
	if updater.updateCalls != 1 || updater.rollbackCalls != 0 {
		t.Fatalf("update calls=%d rollback calls=%d", updater.updateCalls, updater.rollbackCalls)
	}
}

func TestUpdateRollbackReturnsLocalRollbackError(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Updater = &fakeUpdater{rollbackErr: errors.New("no previous portable AIGW binary is available")}
	err := execute(t, app, "update", "--rollback")
	if err == nil || !strings.Contains(err.Error(), "no previous portable AIGW binary") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateHelpDescribesOfflineProgramRollbackInChinese(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "update", "--help"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "离线回退便携版 AIGW 程序到上一版本") {
		t.Fatalf("help = %s", out.String())
	}
}

func TestCriticalCommandHelpUsesChineseGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cases := []struct {
		args []string
		want []string
	}{
		{args: []string{"setup", "--help"}, want: []string{"服务账户标识", "首个模型配置标识", "从标准输入读取一行 Token"}},
		{args: []string{"verify", "--help"}, want: []string{"验证 Claude、Codex 或全部", "验证指定配置，且不修改路由"}},
		{args: []string{"rollback", "--help"}, want: []string{"仅恢复紧邻的一份配置备份"}},
		{args: []string{"config", "import", "--help"}, want: []string{"合并不含密钥的团队清单", "显式替换冲突的服务账户元数据", "系统 Token 保持不变"}},
		{args: []string{"adapter", "auth", "--help"}, want: []string{"将当前服务账户的 Token 绑定到 Codex"}},
	}
	for _, tc := range cases {
		out.Reset()
		if err := execute(t, app, tc.args...); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		help := out.String()
		for _, want := range tc.want {
			if !strings.Contains(help, want) {
				t.Fatalf("%v help missing %q:\n%s", tc.args, want, help)
			}
		}
	}
}

func TestCommonCommandFailuresUseChineseGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	for _, tc := range []struct {
		args []string
		want string
		fix  string
	}{
		{args: []string{"config"}, want: "请选择 config 子命令；运行 `aigw config --help` 查看帮助", fix: "aigw config --help"},
		{args: []string{"adapter", "auth", "claude"}, want: "原生凭据绑定仅适用于 codex", fix: "aigw adapter auth codex"},
		{args: []string{"use", "--for", "other", "one"}, want: "--for 只能是 claude 或 codex", fix: "aigw use --help"},
	} {
		out.Reset()
		err := execute(t, app, tc.args...)
		if err == nil || !strings.Contains(out.String(), tc.want) || !strings.Contains(out.String(), tc.fix) {
			t.Fatalf("%v err=%v output=%s", tc.args, err, out.String())
		}
	}
}

func TestUnknownCommandSuggestsTopLevelHelp(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "not-a-command")
	if err == nil || !strings.Contains(out.String(), "未知命令") || !strings.Contains(out.String(), "aigw --help") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}

func TestUnknownFlagSuggestsTopLevelHelpInChinese(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "status", "--not-a-flag")
	if err == nil || !strings.Contains(out.String(), "未知选项") || !strings.Contains(out.String(), "aigw --help") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}

func TestFailureSuggestionUsesCommandNamedInChineseGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := app.Config.Save(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "setup", "--profile", "new-profile")
	if err == nil || !strings.Contains(out.String(), "AIGW 已完成首次配置") || !strings.Contains(out.String(), "aigw add") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}

func TestCoreValidationFailuresUseChineseGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"test", "--for", "other"}, want: "--for 只能是 claude 或 codex"},
		{args: []string{"verify", "--for", "other"}, want: "--for 只能是 claude、codex 或 all"},
		{args: []string{"setup", "--profile", "new-profile", "--for", "other"}, want: "--for 只能是 claude 或 codex"},
		{args: []string{"profile", "add", "new-profile"}, want: "必须同时提供 --account、--for 和 --model"},
		{args: []string{"route", "reset", "other"}, want: "客户端只能是 claude 或 codex"},
		{args: []string{"adapter", "enable", "other"}, want: "客户端只能是 claude 或 codex"},
	} {
		out.Reset()
		err := execute(t, app, tc.args...)
		if err == nil || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v err=%v output=%s", tc.args, err, out.String())
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
	if err != nil || !strings.Contains(out.String(), "Claude 启动器") || !strings.Contains(out.String(), "AIGW 管理的 Claude 启动器已就绪") {
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
	addAccountProfile(&cfg, "claude", "claude", "Claude", domain.Endpoints{Anthropic: "https://example.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "claude-test"})
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
	addAccountProfile(&cfg, "claude", "claude", "Claude", domain.Endpoints{Anthropic: "https://example.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "claude-test"})
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
	cfg.Profiles["gpt-5.6-sol"] = domain.Profile{Label: "GPT Profile", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
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
	cfg.Profiles["gpt-5.6-sol"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol"}}
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-fable-5"}}
	cfg.Routes.Default = "gpt-5.6-sol"
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
	for _, want := range []string{"Claude", "未选择 Claude 配置", "aigw use claude-fable-5 --for claude"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status lacks %q:\n%s", want, text)
		}
	}
}

func TestTerminalErrorLocalizesResolvedProfileClientMismatch(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "team", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "test", "--for", "claude")
	if err == nil {
		t.Fatal("test command unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"配置 \"gpt\" 仅适用于 codex，不能用于 claude", "建议操作", "aigw check"} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized terminal error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "profile \"gpt\" is for codex, not claude") {
		t.Fatalf("terminal leaked raw domain error:\n%s", text)
	}
	if strings.Contains(text, "连接测试") {
		t.Fatalf("failed test command emitted partial success view:\n%s", text)
	}
}

func TestTestCommandExplainsUnconfiguredStateBeforeResolvingRoutes(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "test", "--for", "claude")
	if err == nil {
		t.Fatal("test command unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"尚未配置", "尚未创建任何服务配置。", "aigw setup"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unconfigured test output lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"连接测试", `unknown profile ""`} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("unconfigured test output retained %q:\n%s", unwanted, text)
		}
	}
}

func TestTerminalErrorLocalizesUnsupportedConfigVersion(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := os.WriteFile(app.Config.Path(), []byte("version = 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "status")
	if err == nil {
		t.Fatal("status unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"配置版本不受支持：当前为 0，要求 2", "建议操作", "aigw check"} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized configuration error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unsupported config version") {
		t.Fatalf("terminal leaked raw configuration error:\n%s", text)
	}
}

func TestStatusWarnsWhenEnabledClaudeAdapterHasNoOwnedShim(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude Fable", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-fable-5"}}
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
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude Fable", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-fable-5"}}
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
	cfg.Profiles["gpt-5.6-sol"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
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
	cfg.Profiles["gpt-5.6-sol"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
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
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "future", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
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
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "future", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
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

func TestUnconfiguredCommandsPointToSetupWithoutLoops(t *testing.T) {
	for _, command := range [][]string{{"status"}, {"check"}, {"repair"}, {"models"}, {"catalog"}} {
		app, out, _, _ := testApp(t, "")
		err := execute(t, app, command...)
		if command[0] == "status" && err != nil {
			t.Fatalf("%v error = %v", command, err)
		}
		if command[0] != "status" && err == nil {
			t.Fatalf("%v succeeded without configuration", command)
		}
		text := out.String() + "\n"
		if err != nil {
			text += err.Error()
		}
		if !strings.Contains(text, "aigw setup") {
			t.Fatalf("%v should point to setup:\n%s", command, text)
		}
		if strings.Contains(text, "运行 `aigw`") || strings.Contains(text, "aigw repair") || strings.Contains(text, "aigw check") {
			t.Fatalf("%v retained a loop or ambiguous first-use action:\n%s", command, text)
		}
	}
}

func TestCatalogUnconfiguredPointsToSetup(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "catalog")
	if err == nil {
		t.Fatal("catalog succeeded without configuration")
	}
	text := out.String() + "\n" + err.Error()
	if !strings.Contains(text, "aigw setup") || strings.Contains(text, "aigw profile add") {
		t.Fatalf("catalog should direct first use to setup:\n%s", text)
	}
}

func TestCatalogJSONUnconfiguredIsEmptyAndMachineReadable(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "catalog", "--json"); err != nil {
		t.Fatalf("catalog --json error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "{\n  \"accounts\": []\n}" {
		t.Fatalf("catalog --json = %q", out.String())
	}
}
