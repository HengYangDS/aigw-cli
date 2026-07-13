package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestDoctorDetectsCodexProjectionDrift(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{Label: "GPT 5.6 Sol Codex", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol"}}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = profile
	cfg.Routes.Default = "gpt-5.6-sol"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapters.SyncCodexConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(data), `model = "gpt-5.6-sol" # managed by AIGW`, `model = "gpt-5.6-terra" # managed by AIGW`, 1)
	if err := os.WriteFile(target, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	err = execute(t, app, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"codex:target-1"`) || !strings.Contains(out.String(), "model selection") || !strings.Contains(out.String(), `"ok": false`) {
		t.Fatalf("doctor output = %s", out.String())
	}
}

func TestDoctorReportsGlobalClientTokenEnvironmentWithoutLeakingValue(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	const secret = "doctor-environment-token-must-not-appear"
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=" + secret}

	if err := execute(t, app, "doctor", "--json"); err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	result := out.String()
	if !strings.Contains(result, `"name": "environment:client-token"`) ||
		!strings.Contains(result, "ANTHROPIC_AUTH_TOKEN") ||
		!strings.Contains(result, `"ok": false`) {
		t.Fatalf("doctor output = %s", result)
	}
	if strings.Contains(result, secret) {
		t.Fatalf("doctor leaked environment token: %s", result)
	}
}

func TestDoctorHumanOutputUsesConciseCheckLabels(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=test-token"}
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor succeeded despite a global client token")
	}
	result := out.String()
	if !strings.Contains(result, "客户端令牌环境") || strings.Contains(result, "environment:client-token") {
		t.Fatalf("doctor human label = %s", result)
	}
}

func TestDoctorHumanOutputTranslatesSuccessfulImplementationDetails(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", domain.Endpoints{Anthropic: "https://team.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "claude-test"})
	cfg.Routes.Default = "team"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.Shims.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	app.Shims.Home = t.TempDir()
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor should report missing shell activation")
	}
	result := out.String()
	for _, want := range []string{"未检测到全局客户端令牌环境变量", "配置有效", "系统密钥", "team · 可用", "已启用", "AIGW 管理的 Claude 启动器", "Claude PATH 激活缺失"} {
		if !strings.Contains(result, want) {
			t.Fatalf("doctor human output missing %q:\n%s", want, result)
		}
	}
	for _, unwanted := range []string{"no global client token environment variables", "config            valid", "available", "AIGW managed shim", "path:claude", "shim:claude"} {
		if strings.Contains(result, unwanted) {
			t.Fatalf("doctor human output leaked implementation prose %q:\n%s", unwanted, result)
		}
	}
}

func TestDoctorHumanOutputTranslatesCodexProjectionFailureButJSONStaysDiagnostic(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	profile := domain.Profile{Label: "GPT", Account: "team", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1"}}
	cfg.Profiles["gpt"] = profile
	cfg.Routes.Default = "gpt"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor should report Codex projection drift")
	}
	human := out.String()
	if !strings.Contains(human, "Codex 配置目标 1") || !strings.Contains(human, "与当前路由不一致") || strings.Contains(human, "Codex config AIGW state is missing") {
		t.Fatalf("doctor human output = %s", human)
	}
	out.Reset()
	if err := execute(t, app, "doctor", "--json"); err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"codex:target-1"`) || !strings.Contains(out.String(), "Codex config AIGW state is missing") {
		t.Fatalf("doctor JSON diagnostic changed = %s", out.String())
	}
}

func TestDoctorHumanOutputNeverExposesRawEnvironmentFixText(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=test-token"}
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor succeeded despite a global client token")
	}
	result := out.String()
	if !strings.Contains(result, "从启动当前终端的父环境中移除上述变量") {
		t.Fatalf("doctor environment fix = %s", result)
	}
	if strings.Contains(result, "remove them from the parent environment") {
		t.Fatalf("doctor leaked raw environment fix = %s", result)
	}
}

func TestDoctorHumanOutputTranslatesUnreadableConfigWithoutLeakingPath(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := os.MkdirAll(app.Config.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor should fail when config path is a directory")
	}
	result := out.String()
	if !strings.Contains(result, "无法读取或校验配置") || !strings.Contains(result, "检查或恢复本机配置文件") {
		t.Fatalf("doctor config failure output = %s", result)
	}
	if strings.Contains(result, app.Config.Path()) || strings.Contains(result, "read") {
		t.Fatalf("doctor leaked raw config error details = %s", result)
	}
}

func TestDoctorJSONKeepsMachineDiagnosticValues(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", domain.Endpoints{Anthropic: "https://team.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "claude-test"})
	cfg.Routes.Default = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "doctor", "--json"); err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	for _, want := range []string{`"name": "config"`, `"detail": "valid"`, `"name": "secret:team"`, `"detail": "available"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("doctor JSON lost machine value %q:\n%s", want, out.String())
		}
	}
}

func TestDoctorUnconfiguredPointsToSetupNotRepair(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor succeeded without configuration")
	}
	result := out.String()
	if !strings.Contains(result, "aigw setup") {
		t.Fatalf("doctor should point to setup:\n%s", result)
	}
	if strings.Contains(result, "aigw repair") {
		t.Fatalf("doctor should not suggest repair before setup:\n%s", result)
	}
}
