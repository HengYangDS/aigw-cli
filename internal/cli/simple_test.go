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
	cfg.Profiles["one"] = domain.Profile{Label: "One Gateway", Endpoints: domain.Endpoints{Anthropic: "https://one.test", OpenAIResponses: "https://one.test/v1"}}
	cfg.Profiles["two"] = domain.Profile{Label: "Two Gateway", Endpoints: domain.Endpoints{Anthropic: "https://two.test", OpenAIResponses: "https://two.test/v1"}}
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
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Routes.Default = "dmx"
	cfg.Adapters["claude"] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
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
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}}
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
