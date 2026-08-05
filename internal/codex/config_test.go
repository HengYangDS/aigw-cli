package codex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
)

func codexRuntime(profileID, label, endpoint, model string) configuration.Runtime {
	return configuration.Runtime{
		ProfileID:    profileID,
		ProfileLabel: label,
		AccountID:    "dmx",
		Client:       configuration.ClientCodex,
		Endpoint:     endpoint,
		Model:        model,
	}
}

func TestCodexSyncProjectsOwnedProviderAndPreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "configuration.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-test\"\n\n[model_providers.native]\nbase_url = \"https://native.test/v1\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("dmx", "DMXAPI", "https://example.test/v1", "gpt-test")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{`model_provider = "aigw" # managed by AIGW`, `model = "gpt-test" # managed by AIGW`, `[agents]`, `max_concurrent_threads_per_session = 16`, `max_depth = 1`, `[features.multi_agent_v2]`, `[model_providers.aigw]`, `base_url = "https://example.test/v1"`, `[model_providers.native]`} {
		if !strings.Contains(text, want) {
			t.Errorf("projected config lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "secret") {
		t.Fatalf("config contains secret-like fixture: %s", text)
	}
	if err := codex.DisableConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if string(restored) != original {
		t.Fatalf("restored config differs\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexSyncProjectsCurrentPerSessionConcurrencySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\nmax_threads = 99\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt", "GPT", "https://example.test/v1", "gpt-test")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"[agents]\n",
		"max_concurrent_threads_per_session = 16\n",
		"max_depth = 1\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Codex config lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "max_concurrent_threads_per_session") != 2 {
		t.Fatalf("Codex config does not bind both scheduler generations:\n%s", text)
	}
	if strings.Contains(text, "max_threads") {
		t.Fatalf("AIGW retained the retired scheduler key:\n%s", text)
	}
	if err := codex.DisableConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != "model_provider = \"native\"\n" {
		t.Fatalf("disable restored the retired scheduler key:\n%s", restored)
	}
}

func TestCodexDisableStopsWhenManagedSelectionWasEdited(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := codexRuntime("dmx", "DMX", "https://example.test/v1", "")
	if err := codex.SyncConfig(path, p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	edited := strings.Replace(string(data), `model_provider = "aigw" # managed by AIGW`, `model_provider = "other"`, 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	err := codex.DisableConfig(path)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("error = %v", err)
	}
}

func TestCodexSyncAndDisablePreserveUnrelatedUserEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("dmx", "DMX", "https://one.test/v1", "gpt-one")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	edited := strings.Replace(string(data), `model = "gpt-original"`, `model = "gpt-user-edit"`, 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	profile.Endpoint = "https://two.test/v1"
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatalf("unrelated user edit blocked sync: %v", err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), `model = "gpt-one" # managed by AIGW`) || !strings.Contains(string(data), `base_url = "https://two.test/v1"`) {
		t.Fatalf("sync lost managed model or endpoint update:\n%s", data)
	}
	if err := codex.DisableConfig(path); err != nil {
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

func TestLoginPlanPassesTokenOnStdinNotArguments(t *testing.T) {
	plan, err := codex.LoginPlan("/usr/local/bin/codex", "/tmp/codex-home", "top-secret")
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

func TestSyncConfigProjectsModelWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6", "GPT-5.6", "https://example.test/v1", "gpt-5.6")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "model = \"gpt-5.6\"") {
		t.Fatalf("Codex config lacks model:\n%s", data)
	}
}

func TestCodexSyncOwnsTopLevelModelAndRestoresOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-sol", "GPT", "https://example.test/v1", "gpt-5.6-sol")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `model = "gpt-5.6-sol" # managed by AIGW`) {
		t.Fatalf("top-level model not managed by selected profile:\n%s", data)
	}
	if strings.Contains(string(data), `[model_providers.aigw]\nmodel =`) {
		t.Fatalf("provider block should not carry model selection:\n%s", data)
	}
	if err := codex.DisableConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if string(restored) != original {
		t.Fatalf("restore mismatch\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexSyncRepairsOnlyAnExactTruncatedOwnedProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-terra", "GPT-5.6 Terra Codex", "https://example.test/v1", "gpt-5.6-terra")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	truncated := strings.Replace(string(projected), "# <<< AIGW managed provider <<<\n", "", 1)
	if err := os.WriteFile(path, []byte(truncated), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatalf("SyncConfig() did not repair the exact owned truncation: %v", err)
	}
	repaired, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repaired), "# <<< AIGW managed provider <<<\n") {
		t.Fatalf("repaired projection is still incomplete:\n%s", repaired)
	}
	if err := codex.ValidateConfig(path, profile); err != nil {
		t.Fatalf("ValidateConfig() rejected repaired projection: %v", err)
	}
}

func TestCodexSyncRefusesATruncatedProjectionWithExtraContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-terra", "GPT-5.6 Terra Codex", "https://example.test/v1", "gpt-5.6-terra")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	truncated := strings.Replace(string(projected), "# <<< AIGW managed provider <<<\n", "foreign = \"do-not-overwrite\"\n", 1)
	if err := os.WriteFile(path, []byte(truncated), 0o600); err != nil {
		t.Fatal(err)
	}

	err = codex.SyncConfig(path, profile)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("SyncConfig() error = %v, want incomplete owned-projection conflict", err)
	}
}

func TestCodexSyncRepairsExactTruncationBeforeUnrelatedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-terra", "GPT-5.6 Terra Codex", "https://example.test/v1", "gpt-5.6-terra")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	truncated := strings.Replace(string(projected), "# <<< AIGW managed provider <<<\n", "", 1)
	truncated += "\n[mcp_servers.node_repl]\ncommand = \"node\"\nenabled = true\n"
	if err := os.WriteFile(path, []byte(truncated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatalf("SyncConfig() did not repair an exact truncated table with preserved tail: %v", err)
	}
	repaired, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repaired), "# <<< AIGW managed provider <<<\n") || !strings.Contains(string(repaired), "[mcp_servers.node_repl]\ncommand = \"node\"") {
		t.Fatalf("sync did not preserve both the repaired block and unrelated tail:\n%s", repaired)
	}
	if err := codex.ValidateConfig(path, profile); err != nil {
		t.Fatalf("ValidateConfig() rejected repaired projection: %v", err)
	}
}

func TestValidateConfigDetectsManagedModelDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-sol", "GPT 5.6 Sol Codex", "https://example.test/v1", "gpt-5.6-sol")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(data), `model = "gpt-5.6-sol" # managed by AIGW`, `model = "gpt-5.6-terra" # managed by AIGW`, 1)
	if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	err = codex.ValidateConfig(path, profile)
	if err == nil || !strings.Contains(err.Error(), "model selection") {
		t.Fatalf("ValidateConfig() error = %v, want managed model drift", err)
	}
}

func TestCodexSyncAcceptsFormatterPaddingOnManagedSelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-sol", "GPT 5.6 Sol Codex", "https://example.test/v1", "gpt-5.6-sol")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	padded := strings.Replace(string(data), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"                                                     # managed by AIGW`, 1)
	padded = strings.Replace(padded, `model = "gpt-5.6-sol" # managed by AIGW`, `model = "gpt-5.6-sol"                                                   # managed by AIGW`, 1)
	if err := os.WriteFile(path, []byte(padded), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := codex.ValidateConfig(path, profile); err != nil {
		t.Fatalf("ValidateConfig() rejected formatter-only padding: %v", err)
	}
	profile.Endpoint = "https://updated.test/v1"
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatalf("SyncConfig() rejected formatter-only padding: %v", err)
	}
	if err := codex.DisableConfig(path); err != nil {
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

func TestCodexSyncRejectsUnmarkedProjectionWhenProviderSemanticsDiffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-sol", "GPT 5.6 Sol Codex", "https://example.test/v1", "gpt-5.6-sol")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.Replace(string(projected), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"`, 1)
	stripped = strings.Replace(stripped, `model = "gpt-5.6-sol" # managed by AIGW`, `model = "gpt-5.6-sol"`, 1)
	stripped = strings.Replace(stripped, "# >>> AIGW managed provider >>>\n", "", 1)
	stripped = strings.Replace(stripped, "# <<< AIGW managed provider <<<\n", "", 1)
	stripped = strings.Replace(stripped, `base_url = "https://example.test/v1"`, `base_url = "https://different.test/v1"`, 1)
	if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
		t.Fatal(err)
	}

	err = codex.SyncConfig(path, profile)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("error = %v, want semantic conflict", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != stripped {
		t.Fatal("SyncConfig changed an unmarked projection with different provider semantics")
	}
}

func TestCodexValidationAndDisablePreserveForeignFieldsBeforeProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-original\"\n\n[mcp_servers.node_repl]\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-sol", "GPT 5.6 Sol Codex", "https://example.test/v1", "gpt-5.6-sol")
	if err := codex.SyncConfig(path, profile); err != nil {
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

	if err := codex.ValidateConfig(path, profile); err != nil {
		t.Fatalf("ValidateConfig() = %v, want foreign fields to be ignored", err)
	}
	if err := codex.DisableConfig(path); err != nil {
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
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt-5.6-sol", "GPT", "https://example.test/v1", "gpt-5.6-sol")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	if err := codex.DisableConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if string(restored) != original {
		t.Fatalf("restore mismatch\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexResyncPreservesAnEmptyOriginalModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	first := codexRuntime("gpt-one", "GPT One", "https://example.test/v1", "gpt-one")
	if err := codex.SyncConfig(path, first); err != nil {
		t.Fatal(err)
	}
	second := codexRuntime("gpt-two", "GPT Two", "https://example.test/v1", "gpt-two")
	if err := codex.SyncConfig(path, second); err != nil {
		t.Fatal(err)
	}
	if err := codex.DisableConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("resync changed an originally empty model selection\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexResyncRefusesUnmarkedManagedProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt", "GPT", "https://example.test/v1", "gpt-test")
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	unmarked := strings.ReplaceAll(string(projected), "# >>> AIGW managed provider >>>\n", "")
	unmarked = strings.ReplaceAll(unmarked, "# <<< AIGW managed provider <<<\n", "")
	if err := os.WriteFile(path, []byte(unmarked), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(path, profile); err == nil || !strings.Contains(err.Error(), "provider block is missing") {
		t.Fatalf("unmarked projection sync error = %v, want missing provider block conflict", err)
	}
}

func envMap(values []string) map[string]string {
	out := map[string]string{}
	for _, value := range values {
		key, v, ok := strings.Cut(value, "=")
		if ok {
			out[key] = v
		}
	}
	return out
}
