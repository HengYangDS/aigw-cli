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
[profiles.team]
label = "Team Gateway"
[profiles.team.endpoints]
anthropic = "https://team.test"
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

func TestLegacyMigrationImportsStructureWithoutChangingSecrets(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	_ = secretStore.Set("dmx", "existing-keyring-secret")
	legacyPath := filepath.Join(t.TempDir(), "config.json")
	legacy := `{"version":2,"profiles":{"dmx":{"label":"DMXAPI","base_url":"https://dmx.test/v1","adapters":{"claude":{"base_url":"https://dmx.test"},"codex":{"base_url":"https://dmx.test/v1"}}}},"routes":{"default":"dmx","claude":"dmx","codex":"dmx"}}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "config", "migrate", legacyPath); err != nil {
		t.Fatal(err)
	}
	got, err := secretStore.Get("dmx")
	if err != nil || got != "existing-keyring-secret" {
		t.Fatalf("secret changed: %q, %v", got, err)
	}
	cfg, _ := app.Config.Load()
	if cfg.Profiles["dmx"].Endpoints.Anthropic != "https://dmx.test" {
		t.Fatalf("migration = %#v", cfg)
	}
}

func TestAdapterEnableClaudeStoresOnlyClaudeExecutable(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims = shims.Manager{GOOS: "linux", BinDir: shimDir, AIGWExecutable: filepath.Join(shimDir, "aigw")}
	cfg := domain.NewConfig()
	cfg.Profiles["team"] = domain.Profile{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
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
	cfg.Profiles["team"] = domain.Profile{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1"}}
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

func TestCodexSyncLogsIntoEachConfiguredHome(t *testing.T) {
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
	cfg.Profiles["team"] = domain.Profile{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1"}}
	cfg.Routes.Default = "team"
	cfg.Adapters["codex"] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex-real", Targets: targets}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 2 {
		t.Fatalf("login plans = %#v", runner.plans)
	}
	for i, target := range targets {
		if processEnvMap(runner.plans[i].Env)["CODEX_HOME"] != filepath.Dir(target) {
			t.Fatalf("plan %d CODEX_HOME = %q", i, processEnvMap(runner.plans[i].Env)["CODEX_HOME"])
		}
	}
}

func TestRunClaudeResolvesRouteAndBuildsProcessPlan(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Profiles["team"] = domain.Profile{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
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
	cfg.Profiles["team"] = domain.Profile{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes.Default = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "profile", "remove", "team")
	if err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("error = %v", err)
	}
}
