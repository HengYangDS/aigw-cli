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
	profile := domain.Profile{ID: "dmx", Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}, Models: domain.Models{Codex: "gpt-test"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{`model_provider = "aigw" # managed by AIGW`, `model = "gpt-test" # managed by AIGW`, `[model_providers.aigw]`, `base_url = "https://example.test/v1"`, `[model_providers.native]`} {
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
	profile := domain.Profile{ID: "dmx", Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, Models: domain.Models{Codex: "gpt-one"}}
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
	if !strings.Contains(string(data), `model = "gpt-one" # managed by AIGW`) || !strings.Contains(string(data), `base_url = "https://two.test/v1"`) {
		t.Fatalf("sync lost managed model or endpoint update:\n%s", data)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), `model_provider = "native"`) || !strings.Contains(string(data), `model = "gpt-original"`) {
		t.Fatalf("disable did not restore original provider/model:\n%s", data)
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

func TestSyncCodexConfigProjectsModelWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "gpt-5.6", Label: "GPT-5.6", Account: "dmx", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}, Models: domain.Models{Codex: "gpt-5.6"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "model = \"gpt-5.6\"") {
		t.Fatalf("Codex config lacks model:\n%s", data)
	}
}

func TestCodexSyncOwnsTopLevelModelAndRestoresOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "gpt-5.6-sol-cdx", Label: "GPT", Account: "dmx", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `model = "gpt-5.6-sol-cdx" # managed by AIGW`) {
		t.Fatalf("top-level model not managed by selected profile:\n%s", data)
	}
	if strings.Contains(string(data), `[model_providers.aigw]\nmodel =`) {
		t.Fatalf("provider block should not carry model selection:\n%s", data)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if string(restored) != original {
		t.Fatalf("restore mismatch\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestValidateCodexConfigDetectsManagedModelDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{
		ID:        "gpt-5.6-sol-cdx",
		Label:     "GPT 5.6 Sol Codex",
		Account:   "dmx",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"},
		Models:    domain.Models{Codex: "gpt-5.6-sol-cdx"},
	}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(data), `model = "gpt-5.6-sol-cdx" # managed by AIGW`, `model = "gpt-5.6-terra" # managed by AIGW`, 1)
	if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	err = adapters.ValidateCodexConfig(path, profile)
	if err == nil || !strings.Contains(err.Error(), "model selection") {
		t.Fatalf("ValidateCodexConfig() error = %v, want managed model drift", err)
	}
}

func TestCodexSyncAcceptsFormatterPaddingOnManagedSelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{
		ID:        "gpt-5.6-sol-cdx",
		Label:     "GPT 5.6 Sol Codex",
		Account:   "dmx",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"},
		Models:    domain.Models{Codex: "gpt-5.6-sol-cdx"},
	}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	padded := strings.Replace(string(data), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"                                                     # managed by AIGW`, 1)
	padded = strings.Replace(padded, `model = "gpt-5.6-sol-cdx" # managed by AIGW`, `model = "gpt-5.6-sol-cdx"                                                   # managed by AIGW`, 1)
	if err := os.WriteFile(path, []byte(padded), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := adapters.ValidateCodexConfig(path, profile); err != nil {
		t.Fatalf("ValidateCodexConfig() rejected formatter-only padding: %v", err)
	}
	profile.Endpoints.OpenAIResponses = "https://updated.test/v1"
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatalf("SyncCodexConfig() rejected formatter-only padding: %v", err)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("restore mismatch\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexSyncRecoversSemanticallyEquivalentProjectionWhoseOwnershipMarkersWereStripped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n\n[features]\nkeep = true\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{
		ID:        "gpt-5.6-sol-cdx",
		Label:     "GPT 5.6 Sol Codex",
		Account:   "dmx",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"},
		Models:    domain.Models{Codex: "gpt-5.6-sol-cdx"},
	}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.Replace(string(projected), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"`, 1)
	stripped = strings.Replace(stripped, `model = "gpt-5.6-sol-cdx" # managed by AIGW`, `model = "gpt-5.6-sol-cdx"`, 1)
	stripped = strings.Replace(stripped, "# >>> AIGW managed provider >>>\n", "", 1)
	stripped = strings.Replace(stripped, "# <<< AIGW managed provider <<<\n", "", 1)
	stripped += "# user note after provider\n"
	if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := adapters.ValidateCodexConfig(path, profile); err == nil {
		t.Fatal("ValidateCodexConfig() accepted a projection whose AIGW ownership markers were stripped")
	}

	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatalf("SyncCodexConfig() did not recover an otherwise equivalent unmarked projection: %v", err)
	}
	recovered, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(recovered), "# user note after provider") {
		t.Fatal("SyncCodexConfig() removed a user comment adjacent to the unmarked provider table")
	}
	if err := adapters.ValidateCodexConfig(path, profile); err != nil {
		t.Fatalf("ValidateCodexConfig() = %v after recovery", err)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restored), "# user note after provider") {
		t.Fatal("DisableCodexConfig() removed the preserved user comment")
	}
	if !strings.Contains(string(restored), original) {
		t.Fatalf("restore lost original configuration\nwant subset:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexSyncRejectsUnmarkedProjectionWhenProviderSemanticsDiffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{
		ID:        "gpt-5.6-sol-cdx",
		Label:     "GPT 5.6 Sol Codex",
		Account:   "dmx",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"},
		Models:    domain.Models{Codex: "gpt-5.6-sol-cdx"},
	}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.Replace(string(projected), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"`, 1)
	stripped = strings.Replace(stripped, `model = "gpt-5.6-sol-cdx" # managed by AIGW`, `model = "gpt-5.6-sol-cdx"`, 1)
	stripped = strings.Replace(stripped, "# >>> AIGW managed provider >>>\n", "", 1)
	stripped = strings.Replace(stripped, "# <<< AIGW managed provider <<<\n", "", 1)
	stripped = strings.Replace(stripped, `base_url = "https://example.test/v1"`, `base_url = "https://different.test/v1"`, 1)
	if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
		t.Fatal(err)
	}

	err = adapters.SyncCodexConfig(path, profile)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("error = %v, want semantic conflict", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != stripped {
		t.Fatal("SyncCodexConfig changed an unmarked projection with different provider semantics")
	}
}

func TestCodexSyncRecoversEquivalentCRLFProjectionWhoseOwnershipMarkersWereStripped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\r\nmodel = \"gpt-original\"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{
		ID:        "gpt-5.6-sol-cdx",
		Label:     "GPT 5.6 Sol Codex",
		Account:   "dmx",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"},
		Models:    domain.Models{Codex: "gpt-5.6-sol-cdx"},
	}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.Replace(string(projected), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"`, 1)
	stripped = strings.Replace(stripped, `model = "gpt-5.6-sol-cdx" # managed by AIGW`, `model = "gpt-5.6-sol-cdx"`, 1)
	stripped = strings.Replace(stripped, "# >>> AIGW managed provider >>>\n", "", 1)
	stripped = strings.Replace(stripped, "# <<< AIGW managed provider <<<\n", "", 1)
	stripped = strings.ReplaceAll(stripped, "\n", "\r\n")
	if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatalf("SyncCodexConfig() did not recover an equivalent CRLF projection: %v", err)
	}
	if err := adapters.ValidateCodexConfig(path, profile); err != nil {
		t.Fatalf("ValidateCodexConfig() = %v after CRLF recovery", err)
	}
}

func TestCodexValidationAndDisablePreserveForeignFieldsBeforeProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n\n[mcp_servers.node_repl]\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{
		ID:        "gpt-5.6-sol-cdx",
		Label:     "GPT 5.6 Sol Codex",
		Account:   "dmx",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"},
		Models:    domain.Models{Codex: "gpt-5.6-sol-cdx"},
	}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	foreign := "args = []\ncommand = \"node_repl\"\nenabled = true\n"
	mutated := strings.Replace(string(data), "# >>> AIGW managed provider >>>\n[model_providers.aigw]", "# >>> AIGW managed provider >>>\n"+foreign+"[model_providers.aigw]", 1)
	if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := adapters.ValidateCodexConfig(path, profile); err != nil {
		t.Fatalf("ValidateCodexConfig() = %v, want foreign fields to be ignored", err)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(restored)
	for _, want := range []string{`model_provider = "native"`, `model = "gpt-original"`, foreign} {
		if !strings.Contains(text, want) {
			t.Fatalf("restored config lost %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "AIGW managed provider") || strings.Contains(text, "model_providers.aigw") {
		t.Fatalf("restored config retained AIGW projection:\n%s", text)
	}
}

func TestCodexDisableRemovesManagedModelWhenNoOriginalModelExisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "gpt-5.6-sol-cdx", Label: "GPT", Account: "dmx", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if string(restored) != original {
		t.Fatalf("restore mismatch\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexSyncBackfillsOriginalModelForLegacyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	legacyManaged := `model_provider = "aigw" # managed by AIGW
model = "gpt-original"

# >>> AIGW managed provider >>>
[model_providers.aigw]
name = "AIGW: Old"
base_url = "https://old.test/v1"
wire_api = "responses"
requires_openai_auth = true
# <<< AIGW managed provider <<<
`
	if err := os.WriteFile(path, []byte(legacyManaged), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".aigw-state.json", []byte(`{"original_provider":"model_provider = \"DMX1\"","managed_block_hash":"3b6be2527ed1e77a9a1e5092af165de1a9e7c76289e65fe3bdfad6bf72dee9bd"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "gpt-5.6-sol-cdx", Label: "GPT", Account: "dmx", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	state, _ := os.ReadFile(path + ".aigw-state.json")
	if !strings.Contains(string(state), `"original_model": "model = \"gpt-original\""`) {
		t.Fatalf("state did not backfill original model:\n%s", state)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if !strings.Contains(string(restored), `model_provider = "DMX1"`) || !strings.Contains(string(restored), `model = "gpt-original"`) {
		t.Fatalf("legacy disable did not restore provider/model:\n%s", restored)
	}
}

func TestCodexSyncBackfillsCurrentModelWithoutManagedMarkerWhenOriginalWasLost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	legacyManaged := `model = "gpt-5.6-sol-cdx" # managed by AIGW
model_provider = "aigw" # managed by AIGW

# >>> AIGW managed provider >>>
[model_providers.aigw]
name = "AIGW: Old"
base_url = "https://old.test/v1"
wire_api = "responses"
requires_openai_auth = true
# <<< AIGW managed provider <<<
`
	if err := os.WriteFile(path, []byte(legacyManaged), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".aigw-state.json", []byte(`{"original_provider":"model_provider = \"DMX1\"","managed_block_hash":"3b6be2527ed1e77a9a1e5092af165de1a9e7c76289e65fe3bdfad6bf72dee9bd"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "gpt-5.6-terra-cdx", Label: "GPT", Account: "dmx", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}, Models: domain.Models{Codex: "gpt-5.6-terra-cdx"}}
	if err := adapters.SyncCodexConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	state, _ := os.ReadFile(path + ".aigw-state.json")
	if !strings.Contains(string(state), `"original_model": "model = \"gpt-5.6-sol-cdx\""`) || strings.Contains(string(state), "managed by AIGW") {
		t.Fatalf("state did not backfill a clean fallback model:\n%s", state)
	}
	if err := adapters.DisableCodexConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if strings.Contains(string(restored), "AIGW managed") || !strings.Contains(string(restored), `model = "gpt-5.6-sol-cdx"`) {
		t.Fatalf("disable did not restore clean fallback model:\n%s", restored)
	}
}
