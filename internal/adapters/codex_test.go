package adapters_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestCodexSyncProjectsOwnedProviderAndPreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-test\"\n\n[model_providers.native]\nbase_url = \"https://native.test/v1\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "dmx", Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{`model_provider = "aigw" # managed by AIGW`, `[model_providers.aigw]`, `base_url = "https://example.test/v1"`, `model = "gpt-test"`, `[model_providers.native]`} {
		if !strings.Contains(text, want) {
			t.Errorf("projected config lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "secret") {
		t.Fatalf("config contains secret-like fixture: %s", text)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if string(restored) != original {
		t.Fatalf("restored config differs\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexDisableStopsWhenManagedSelectionWasEdited(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := domain.Profile{ID: "dmx", Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	if err := adapters.SyncCodexConfig(path, p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	edited := strings.Replace(string(data), `model_provider = "aigw" # managed by AIGW`, `model_provider = "other"`, 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	err := adapters.DisableCodexConfig(path)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("error = %v", err)
	}
}

func TestCodexSyncAndDisablePreserveUnrelatedUserEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "dmx", Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	edited := strings.Replace(string(data), `model = "gpt-original"`, `model = "gpt-user-edit"`, 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	profile.Endpoints.OpenAIResponses = "https://two.test/v1"
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatalf("unrelated user edit blocked sync: %v", err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), `model = "gpt-user-edit"`) || !strings.Contains(string(data), `base_url = "https://two.test/v1"`) {
		t.Fatalf("sync lost user edit or endpoint update:\n%s", data)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), `model_provider = "native"`) || !strings.Contains(string(data), `model = "gpt-user-edit"`) {
		t.Fatalf("disable lost user-owned content:\n%s", data)
	}
	if strings.Contains(string(data), "AIGW managed") || strings.Contains(string(data), "model_providers.aigw") {
		t.Fatalf("disable left managed content:\n%s", data)
	}
}

func TestCodexLoginPlanPassesTokenOnStdinNotArguments(t *testing.T) {
	plan, err := adapters.CodexLoginPlan("/usr/local/bin/codex", "/tmp/codex-home", "top-secret")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, " ")
	if strings.Contains(joined, "top-secret") || plan.Stdin != "top-secret\n" {
		t.Fatalf("unsafe login plan: %#v", plan)
	}
	if envMap(plan.Env)["CODEX_HOME"] != "/tmp/codex-home" {
		t.Fatalf("env = %#v", plan.Env)
	}
}
