package codex_test

import (
	"aigw-cli/internal/codex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodexSyncRetiresTheAgentsAliasWhenBindingMaxThreads starts from a user
// configuration that already carries the retired [agents] alias. Codex reads
// max_threads as the session concurrency field and the alias as a second
// spelling of it, so a table holding both is rejected outright: projecting
// max_threads therefore has to clear the alias, and disable has to put it back.
func TestCodexSyncRetiresTheAgentsAliasWhenBindingMaxThreads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\nmax_threads = 99\n\n[agents]\nmax_concurrent_threads_per_session = 7\nmax_depth = 3\n\n[features.multi_agent_v2]\nmax_concurrent_threads_per_session = 5\n"
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
		"max_threads = 16 # managed by AIGW",
		"max_depth = 1 # managed by AIGW",
		"[features.multi_agent_v2]\n",
		"max_concurrent_threads_per_session = 16 # managed by AIGW",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Codex config lacks %q:\n%s", want, text)
		}
	}
	agentsTable := text[strings.Index(text, "[agents]"):strings.Index(text, "[features.multi_agent_v2]")]
	if strings.Contains(agentsTable, "max_concurrent_threads_per_session") {
		t.Fatalf("Codex config declares one concurrency field twice in [agents]:\n%s", text)
	}
	if strings.Count(text, "max_concurrent_threads_per_session") != 1 {
		t.Fatalf("Codex config binds the alias outside the feature-gated table:\n%s", text)
	}
	if !strings.Contains(text, "max_threads = 99") {
		t.Fatalf("AIGW changed an unrelated user-owned scheduler key:\n%s", text)
	}
	if strings.Count(text, "[agents]") != 1 || strings.Count(text, "[features.multi_agent_v2]") != 1 {
		t.Fatalf("AIGW duplicated an existing Codex table:\n%s", text)
	}
	// A second sync must be a no-op rather than re-projecting over its own work.
	if err := codex.SyncConfig(path, profile); err != nil {
		t.Fatal(err)
	}
	repeated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(repeated) != text {
		t.Fatalf("repeated sync changed the projection:\nfirst:\n%s\nsecond:\n%s", text, repeated)
	}
	if err := codex.ValidateConfig(path, profile); err != nil {
		t.Fatalf("validation rejected AIGW's own projection: %v", err)
	}
	if err := codex.DisableConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("disable did not restore the byte-exact user configuration:\n%s", restored)
	}
}

// TestCodexSyncOwnsMaxThreadsWithoutOwningItsNeighbors starts from a user
// configuration whose [agents].max_threads is already the legal spelling. AIGW
// now owns that key, so the projection replaces the value and disable restores
// it byte for byte, while a neighbouring key in the same table is never touched.
func TestCodexSyncOwnsMaxThreadsWithoutOwningItsNeighbors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\n\n[agents]\nmax_threads = 9\nnotify = true\n"
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
	if strings.Count(text, "[agents]") != 1 || strings.Count(text, "[features.multi_agent_v2]") != 1 {
		t.Fatalf("AIGW did not project one scheduler table of each kind:\n%s", text)
	}
	if !strings.Contains(text, "max_threads = 16 # managed by AIGW") {
		t.Fatalf("AIGW did not take ownership of the legal concurrency key:\n%s", text)
	}
	if strings.Contains(text, "max_threads = 9") {
		t.Fatalf("AIGW left a second concurrency value in place:\n%s", text)
	}
	if !strings.Contains(text, "notify = true") {
		t.Fatalf("AIGW changed an unrelated key in an owned table:\n%s", text)
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

func TestCodexSyncRejectsManagedSchedulerDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
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
	drifted := strings.Replace(string(data), "max_depth = 1 # managed by AIGW", "max_depth = 2 # managed by AIGW", 1)
	if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	err = codex.SyncConfig(path, profile)
	if err == nil || !strings.Contains(err.Error(), "scheduler keys changed") {
		t.Fatalf("SyncConfig() error = %v, want scheduler conflict", err)
	}
	current, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(current) != drifted {
		t.Fatal("rejected scheduler drift was overwritten")
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
