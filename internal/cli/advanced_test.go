package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

func TestConfigImportAndExportAreSecretFree(t *testing.T) {
	app, out, secrets, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 1
recommended_default = "team"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team Gateway"
account = "team"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "config", "import", manifestPath); err != nil {
		t.Fatal(err)
	}
	if secrets.Has("team") {
		t.Fatal("manifest import invented a secret")
	}
	out.Reset()
	if err := execute(t, app, "config", "export"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out.String()), "token") || !strings.Contains(out.String(), "Team Gateway") {
		t.Fatalf("unsafe export:\n%s", out.String())
	}
}

func TestConfigDoesNotExposeRemovedLegacyMigration(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	err := execute(t, app, "config", "migrate", filepath.Join(t.TempDir(), "config.json"))
	if err == nil || !strings.Contains(err.Error(), "unknown config command") {
		t.Fatalf("legacy migration command error = %v, want unknown config command", err)
	}
}

func TestAdapterEnableClaudeStoresOnlyClaudeExecutable(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims = shims.Manager{GOOS: "linux", BinDir: shimDir, AIGWExecutable: filepath.Join(shimDir, "aigw")}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", domain.Endpoints{Anthropic: "https://team.test"}, "", domain.Models{})
	cfg.Routes.Default = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := execute(t, app, "adapter", "enable", "claude", "--executable", "/opt/claude-real"); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Adapters["claude"].Enabled || got.Adapters["claude"].Executable != "/opt/claude-real" {
		t.Fatalf("Claude adapter = %#v", got.Adapters["claude"])
	}
	if _, exists := got.Adapters["codex"]; exists {
		t.Fatalf("Claude enable touched Codex: %#v", got.Adapters)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); err != nil {
		t.Fatalf("Claude shim missing: %v", err)
	}
	if err := execute(t, app, "adapter", "disable", "claude"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); !os.IsNotExist(err) {
		t.Fatalf("Claude shim remains: %v", err)
	}
}

func TestAdapterEnableAndDisableCodexOwnsOnlyConfiguredTarget(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	target := filepath.Join(t.TempDir(), "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "model_provider = \"native\"\nmodel = \"gpt-test\"\n"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", domain.Endpoints{OpenAIResponses: "https://team.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "team"
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

func TestCodexSyncReconcilesEachConfiguredHomeWithoutLoggingIn(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	dir := t.TempDir()
	targets := []string{filepath.Join(dir, "one", "config.toml"), filepath.Join(dir, "two", "config.toml")}
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", domain.Endpoints{OpenAIResponses: "https://team.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "team"
	cfg.Adapters["codex"] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex-real", Targets: targets}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync must not start credential binding plans: %#v", runner.plans)
	}
	for _, target := range targets {
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "AIGW managed provider") {
			t.Fatalf("sync did not reconcile %s:\n%s", target, data)
		}
	}
}

func TestAdapterAuthBindsCurrentCodexAccount(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex-real", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "rebind-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "adapter", "auth", "codex"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Stdin != "rebind-token\n" {
		t.Fatalf("auth plans = %#v", runner.plans)
	}
}

func TestRunClaudeResolvesRouteAndBuildsProcessPlan(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", domain.Endpoints{Anthropic: "https://team.test"}, "", domain.Models{})
	cfg.Routes.Default = "team"
	cfg.Adapters["claude"] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "claude-secret")
	if err := cli.RunClaude(app, []string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != "/opt/claude-real" || processEnvMap(runner.plans[0].Env)["ANTHROPIC_AUTH_TOKEN"] != "claude-secret" {
		t.Fatalf("Claude process plan = %#v", runner.plans)
	}
}

func processEnvMap(values []string) map[string]string {
	result := map[string]string{}
	for _, value := range values {
		key, item, ok := strings.Cut(value, "=")
		if ok {
			result[key] = item
		}
	}
	return result
}

func TestProfileRemoveRefusesActiveProfile(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", domain.Endpoints{Anthropic: "https://team.test"}, "", domain.Models{})
	cfg.Routes.Default = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "profile", "remove", "team")
	if err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigImportReportsMissingAccountTokensNotProfileTokens(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 1
recommended_default = "gpt-long-model"
[accounts.dmx]
label = "DMXAPI"
[accounts.dmx.endpoints]
openai_responses = "https://dmx.test/v1"
[profiles."gpt-long-model"]
label = "GPT Long Model"
account = "dmx"
client = "codex"
[profiles."gpt-long-model".models]
codex = "gpt-long-model"
[profiles."claude-long-model"]
label = "Claude Long Model"
account = "dmx"
client = "claude"
[profiles."claude-long-model".models]
claude = "claude-long-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "existing-token")
	if err := execute(t, app, "config", "import", manifestPath); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "需要录入 Token") || strings.Contains(text, "gpt-long-model  ") || strings.Contains(text, "claude-long-model  ") {
		t.Fatalf("import reported profile-level missing tokens despite account token:\n%s", text)
	}
	for _, want := range []string{"账户数量", "系统密钥", "dmx", "Token 可用", "aigw models"} {
		if !strings.Contains(text, want) {
			t.Fatalf("import output lacks %q:\n%s", want, text)
		}
	}
}

func TestConfigImportReportsOnlyMissingAccounts(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 1
recommended_default = "gpt-long-model"
[accounts.dmx]
label = "DMXAPI"
[accounts.dmx.endpoints]
openai_responses = "https://dmx.test/v1"
[profiles."gpt-long-model"]
label = "GPT Long Model"
account = "dmx"
client = "codex"
[profiles."gpt-long-model".models]
codex = "gpt-long-model"
[profiles."claude-long-model"]
label = "Claude Long Model"
account = "dmx"
client = "claude"
[profiles."claude-long-model".models]
claude = "claude-long-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "config", "import", manifestPath); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "dmx") || !strings.Contains(text, "需要录入 Token") || !strings.Contains(text, "aigw rotate dmx") {
		t.Fatalf("import did not point to missing account token:\n%s", text)
	}
	if strings.Contains(text, "gpt-long-model") || strings.Contains(text, "claude-long-model") {
		t.Fatalf("import should not report profile names as missing token slots:\n%s", text)
	}
}

func TestDoctorChecksAccountTokenOnceForSharedProfiles(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["alpha-model"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "alpha-model"}}
	cfg.Profiles["beta-model"] = domain.Profile{Label: "Claude", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{Claude: "beta-model"}}
	cfg.Routes.Default = "alpha-model"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "doctor"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "secret:alpha-model") || strings.Contains(text, "secret:beta-model") {
		t.Fatalf("doctor checked profile secrets instead of account secret:\n%s", text)
	}
	if !strings.Contains(text, "secret:dmx") || !strings.Contains(text, "未发现问题") {
		t.Fatalf("doctor did not report account secret cleanly:\n%s", text)
	}
}

func TestProfileRenameKeepsAccountTokenSlotUnchanged(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-old"] = domain.Profile{Label: "GPT Old", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-old"}}
	cfg.Routes.Default = "gpt-old"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := execute(t, app, "profile", "rename", "gpt-old", "gpt-new"); err != nil {
		t.Fatal(err)
	}
	if got, err := secretStore.Get("dmx"); err != nil || got != "account-token" {
		t.Fatalf("account token changed: %q %v", got, err)
	}
	if secretStore.Has("gpt-new") || secretStore.Has("gpt-old") {
		t.Fatalf("profile rename created profile-level secret slots")
	}
	got, _ := app.Config.Load()
	if got.Routes.Default != "gpt-new" || got.Profiles["gpt-new"].Account != "dmx" {
		t.Fatalf("rename config = %#v", got)
	}
}

func TestProfileRemoveLeavesAccountAndTokenIntact(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-default"] = domain.Profile{Label: "GPT Default", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-default"}}
	cfg.Profiles["gpt-unused"] = domain.Profile{Label: "GPT Unused", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-unused"}}
	cfg.Routes.Default = "gpt-default"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := execute(t, app, "profile", "remove", "gpt-unused"); err != nil {
		t.Fatal(err)
	}
	if got, err := secretStore.Get("dmx"); err != nil || got != "account-token" {
		t.Fatalf("account token changed: %q %v", got, err)
	}
	got, _ := app.Config.Load()
	if _, ok := got.Profiles["gpt-unused"]; ok || got.Accounts["dmx"].Label != "DMXAPI" {
		t.Fatalf("remove config = %#v", got)
	}
}

func TestProfileAddReusesAccountTokenAndLeavesRouteUntouched(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "gpt", "dmx", "GPT", domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}, domain.ClientCodex, domain.Models{Codex: "gpt-test"})
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "existing-account-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "profile", "add", "claude", "--account", "dmx", "--for", "claude", "--model", "claude-test", "--label", "Claude Test"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile := got.Profiles["claude"]
	if profile.Account != "dmx" || profile.Client != domain.ClientClaude || profile.Models.Claude != "claude-test" {
		t.Fatalf("added profile = %#v", profile)
	}
	if got.Routes.Default != "gpt" || !secretStore.Has("dmx") || secretStore.Has("claude") {
		t.Fatalf("route or token slots changed: routes=%#v dmx=%v claude=%v", got.Routes, secretStore.Has("dmx"), secretStore.Has("claude"))
	}
}

func TestAccountEditUpdatesSharedEndpointWithoutProfileDuplication(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "gpt", "dmx", "DMXAPI", domain.Endpoints{OpenAIResponses: "https://old.test/v1", Anthropic: "https://old.test"}, domain.ClientCodex, domain.Models{Codex: "gpt-test"})
	addAccountProfile(&cfg, "claude", "dmx", "DMXAPI", domain.Endpoints{}, domain.ClientClaude, domain.Models{Claude: "claude-test"})
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "account", "edit", "dmx", "--openai-url", "https://new.test/v1"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["dmx"].Endpoints.OpenAIResponses != "https://new.test/v1" {
		t.Fatalf("account endpoint = %#v", got.Accounts["dmx"])
	}
	for _, profile := range got.Profiles {
		if profile.Account != "dmx" {
			t.Fatalf("shared profile lost account reference: %#v", profile)
		}
	}
}

func TestProfileAddRejectsClientWithoutMatchingAccountEndpoint(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "gpt", "openai-only", "OpenAI Only", domain.Endpoints{OpenAIResponses: "https://openai.test/v1"}, domain.ClientCodex, domain.Models{Codex: "gpt-test"})
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "profile", "add", "claude", "--account", "openai-only", "--for", "claude", "--model", "claude-test")
	if err == nil || !strings.Contains(err.Error(), "no Anthropic endpoint") {
		t.Fatalf("profile add error = %v", err)
	}
	got, loadErr := app.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if _, exists := got.Profiles["claude"]; exists {
		t.Fatalf("unusable Profile was persisted: %#v", got.Profiles["claude"])
	}
}
