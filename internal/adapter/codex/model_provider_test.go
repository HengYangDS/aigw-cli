package codex_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/platform"
)

// AIGW's provider table authenticates through its credential command, so a
// projection without one, or with one that is not an absolute path, is
// refused before any write.
func TestCodexSyncRefusesAProviderTableWithoutAnAbsoluteCredentialCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := codexRuntime("native", "Native", "https://gateway.test/openai/v1", "wire-model")
	for _, command := range []string{"", "aigw"} {
		runtime.CredentialCommand = command
		if _, err := codex.ReconcileConfigs(nil, []codex.TargetRef{codex.HomeTarget(path)}, runtime, runtime, false); err == nil {
			t.Fatalf("credential command %q was accepted", command)
		}
		if data, err := os.ReadFile(path); err != nil || len(data) != 0 {
			t.Fatalf("a refused projection wrote %q, %v", data, err)
		}
	}
}

func TestCodexValidateResolvesCurrentAIGWExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runtime := codexRuntime("native", "Native", "https://gateway.test/openai/v1", "wire-model")
	runtime.CredentialCommand = executable
	if _, err := codex.ReconcileConfigs(nil, []codex.TargetRef{codex.HomeTarget(path)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}

	runtime.CredentialCommand = ""
	if err := codex.ValidateConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
}

// earlierNativeTable is the provider table that an earlier release wrote for
// a client's explicit model_provider under client-native authentication:
// AIGW's markers around a table that bears another provider's name and no
// credential command.
const earlierNativeTable = "[model_providers.amazon-bedrock]\nbase_url = \"https://gateway.test/openai/v1\"\nwire_api = \"responses\"\n# <<< AIGW managed provider <<<\n"

// TestCodexSyncReturnsToTheDefaultProvider holds the scenario "Return to the
// default provider": AIGW's record names a provider table other than its own,
// which an earlier release wrote for an explicit model_provider, and the next
// sync removes that table in the transaction that writes the aigw table, whose
// credential command Codex then authenticates through. Rolling that one
// transaction back restores the earlier table, and a withdrawal still restores
// the file that the earlier release found.
func TestCodexSyncReturnsToTheDefaultProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-native\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := codexRuntime("dmx", "DMXAPI", "https://gateway.test/openai/v1", "openai.gpt-5.6-sol")
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	target := []codex.TargetRef{codex.HomeTarget(path)}
	if _, err := codex.ReconcileConfigs(nil, target, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	earlier := rewriteAsEarlierNativeProjection(t, path)

	receipt, err := codex.ReconcileConfigs(target, target, runtime, runtime, false)
	if err != nil {
		t.Fatalf("the sync refused the earlier provider table: %v", err)
	}
	text := string(readTestFile(t, path))
	for _, want := range []string{
		`model_provider = "aigw" # managed by AIGW`,
		"[model_providers.aigw]",
		"[model_providers.aigw.auth]",
		`command = "` + runtime.CredentialCommand + `"`,
		runtime.CredentialProjectionFingerprint(codex.ID),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("the sync did not write %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "amazon-bedrock") {
		t.Fatalf("the earlier provider table survived the sync:\n%s", text)
	}
	if record := string(readTestFile(t, path+".aigw-state.json")); strings.Contains(record, "projected_provider") {
		t.Fatalf("the record still names another provider table:\n%s", record)
	}
	if err := codex.ValidateConfig(path, runtime); err != nil {
		t.Fatalf("the converged file does not show the selection: %v", err)
	}

	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	if restored := string(readTestFile(t, path)); restored != earlier {
		t.Fatalf("a rollback of the transaction gave\n%s\nwant\n%s", restored, earlier)
	}
	if _, err := codex.ReconcileConfigs(target, target, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	if _, err := codex.ReconcileConfigs(target, nil, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	if withdrawn := string(readTestFile(t, path)); withdrawn != original {
		t.Fatalf("the withdrawal gave\n%s\nwant\n%s", withdrawn, original)
	}
}

// rewriteAsEarlierNativeProjection turns AIGW's projection at path into the
// one that an earlier release wrote for an explicit model_provider: the root
// selection and the table between AIGW's markers name amazon-bedrock, and the
// record names that table and its hash. It returns the rewritten file.
func rewriteAsEarlierNativeProjection(t *testing.T, path string) string {
	t.Helper()
	text := string(readTestFile(t, path))
	start, end := strings.Index(text, "[model_providers.aigw]"), strings.Index(text, "# <<< AIGW managed provider <<<\n")
	if start < 0 || end < start {
		t.Fatalf("the projection has no table of AIGW's:\n%s", text)
	}
	text = text[:start] + earlierNativeTable + text[end+len("# <<< AIGW managed provider <<<\n"):]
	text = strings.Replace(text, `model_provider = "aigw" # managed by AIGW`, `model_provider = "amazon-bedrock" # managed by AIGW`, 1)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	recordPath := path + ".aigw-state.json"
	var record map[string]any
	if err := json.Unmarshal(readTestFile(t, recordPath), &record); err != nil {
		t.Fatal(err)
	}
	record["projected_provider"] = "amazon-bedrock"
	record["managed_block_hash"] = platform.SHA256([]byte(earlierNativeTable))
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordPath, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return text
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
