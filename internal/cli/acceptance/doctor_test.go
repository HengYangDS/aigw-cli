package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
)

func TestDoctorDetectsCodexProjectionDrift(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := configuration.Profile{Label: "GPT 5.6 Sol Codex", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = profile
	cfg.Routes.Default = "gpt-5.6-sol"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}
	runtime, _, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, runtime); err != nil {
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

func TestDoctorIgnoresRetiredProviderTokenEnvironmentNames(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Env = []string{
		"DMXAPI_TOKEN=retired-token-must-not-appear",
		"DMX_API_TOKEN=retired-token-must-not-appear",
	}

	if err := execute(t, app, "doctor", "--json"); err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	result := out.String()
	if !strings.Contains(result, `"name": "environment:client-token"`) || !strings.Contains(result, `"ok": true`) {
		t.Fatalf("doctor output = %s", result)
	}
	for _, forbidden := range []string{"DMXAPI_TOKEN", "DMX_API_TOKEN", "retired-token-must-not-appear"} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("doctor still treats a retired provider token name as input: %s", result)
		}
	}
}

func TestDoctorHumanOutputUsesConciseCheckLabels(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=test-token"}
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor succeeded despite a global client token")
	}
	result := out.String()
	if !strings.Contains(result, "Client token environment") || strings.Contains(result, "environment:client-token") {
		t.Fatalf("doctor human label = %s", result)
	}
}

func TestDoctorHumanOutputTranslatesSuccessfulImplementationDetails(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "claude-test"})
	cfg.Routes.Default = "team"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	app.ClaudeLauncher.BinDir = shimDir
	app.ClaudeLauncher.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.ClaudeLauncher.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	app.ClaudeLauncher.Home = t.TempDir()
	if err := execute(t, app, "doctor"); err == nil {
		t.Fatal("doctor should report missing shell activation")
	}
	result := out.String()
	for _, want := range []string{"No global client token environment variables detected", "Configuration is valid", "System secret", "team · available", "Enabled", "AIGW-managed Claude launcher", "Claude PATH activation is missing"} {
		if !strings.Contains(result, want) {
			t.Fatalf("doctor human output missing %q:\n%s", want, result)
		}
	}
	for _, unwanted := range []string{"no global client token environment variables", "config            valid", "AIGW managed launcher", "path:claude", "launcher:claude"} {
		if strings.Contains(result, unwanted) {
			t.Fatalf("doctor human output leaked implementation prose %q:\n%s", unwanted, result)
		}
	}
}

func TestDoctorHumanOutputTranslatesCodexProjectionFailureButJSONStaysDiagnostic(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	profile := configuration.Profile{Label: "GPT", Account: "team", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}}
	cfg.Profiles["gpt"] = profile
	cfg.Routes.Default = "gpt"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
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
	if !strings.Contains(human, "Codex configuration target 1") || !strings.Contains(human, "Does not match the current route") || strings.Contains(human, "Codex config AIGW state is missing") {
		t.Fatalf("doctor human output = %s", human)
	}
	if !strings.Contains(human, "Next\n  aigw sync") || strings.Contains(human, "Next\n  aigw repair") {
		t.Fatalf("doctor drift next action = %s", human)
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
	if !strings.Contains(result, "Remove the variables above from the parent environment that launched this terminal") {
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
	if !strings.Contains(result, "Cannot read or validate configuration") || !strings.Contains(result, "Inspect or restore the local configuration file") {
		t.Fatalf("doctor config failure output = %s", result)
	}
	if strings.Contains(result, app.Config.Path()) || strings.Contains(result, "is a directory") {
		t.Fatalf("doctor leaked raw config error details = %s", result)
	}
}

func TestDoctorJSONKeepsMachineDiagnosticValues(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "claude-test"})
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
