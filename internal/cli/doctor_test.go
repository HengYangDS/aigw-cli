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
	profile := domain.Profile{Label: "GPT 5.6 Sol Codex", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol-cdx"}}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = profile
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Targets: []string{target}}
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
	drifted := strings.Replace(string(data), `model = "gpt-5.6-sol-cdx" # managed by AIGW`, `model = "gpt-5.6-terra" # managed by AIGW`, 1)
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
