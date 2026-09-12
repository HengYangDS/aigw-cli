package codex_test

import (
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func codexRuntime(profileID, label, endpoint, model string) configuration.Runtime {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return configuration.Runtime{
		CredentialCommand: executable,
		ProfileID:         profileID,
		ProfileLabel:      label,
		AccountID:         "dmx",
		Client:            configuration.ClientCodex,
		Endpoint:          endpoint,
		Model:             model,
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
	for _, want := range []string{`model_provider = "aigw" # managed by AIGW`, `model = "gpt-test" # managed by AIGW`, `[agents]`, `max_threads = 16`, `max_depth = 1`, `[features.multi_agent_v2]`, `max_concurrent_threads_per_session = 16`, `[model_providers.aigw]`, `base_url = "https://example.test/v1"`, `[model_providers.native]`} {
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

func TestCodexDisablePreservesEarlierProviderTableReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "# See [model_providers.aigw] in the generated section below.\nmodel_provider = \"native\"\nmodel = \"gpt-original\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt", "GPT", "https://example.test/v1", "gpt-test")
	if err := codex.SyncConfig(path, profile); err != nil {
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
		t.Fatalf("disable changed user content around an earlier provider-table reference\nwant:\n%s\ngot:\n%s", original, restored)
	}
}

func TestCodexSyncRejectsMalformedTOMLWithoutChangingFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\n[agents\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := codexRuntime("gpt", "GPT", "https://example.test/v1", "gpt-test")
	err := codex.SyncConfig(path, profile)
	if err == nil || !strings.Contains(err.Error(), "parse Codex config") {
		t.Fatalf("SyncConfig() error = %v, want TOML parse failure", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != original {
		t.Fatalf("malformed config changed\nwant:\n%s\ngot:\n%s", original, data)
	}
	if _, statErr := os.Stat(path + ".aigw-state.json"); !os.IsNotExist(statErr) {
		t.Fatalf("state sidecar exists after rejected projection: %v", statErr)
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

func TestCodexProjectionRejectsUnownedChanges(t *testing.T) {
	profile := codexRuntime("gpt-5.6-sol", "GPT 5.6 Sol Codex", "https://example.test/v1", "gpt-5.6-sol")
	project := func(t *testing.T) (string, string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := codex.SyncConfig(path, profile); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return path, string(data)
	}

	t.Run("foreign content replaces a managed marker", func(t *testing.T) {
		path, projected := project(t)
		truncated := strings.Replace(projected, "# <<< AIGW managed provider <<<\n", "foreign = \"do-not-overwrite\"\n", 1)
		if err := os.WriteFile(path, []byte(truncated), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := codex.SyncConfig(path, profile); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("SyncConfig() error = %v, want incomplete owned-projection conflict", err)
		}
	})

	t.Run("managed model drifts", func(t *testing.T) {
		path, projected := project(t)
		drifted := strings.Replace(projected, `model = "gpt-5.6-sol" # managed by AIGW`, `model = "gpt-5.6-terra" # managed by AIGW`, 1)
		if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := codex.ValidateConfig(path, profile); err == nil || !strings.Contains(err.Error(), "model selection") {
			t.Fatalf("ValidateConfig() error = %v, want managed model drift", err)
		}
	})
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
