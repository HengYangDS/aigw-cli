package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfigReportsCatalogDrift(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("openai.gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	catalogPath := codexCatalogPath(path)
	if err := os.WriteFile(catalogPath, []byte(`{"models":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "model catalog changed") {
		t.Fatalf("ValidateConfig() error = %v, want a catalog conflict", err)
	}
	if err := os.Remove(catalogPath); err != nil {
		t.Fatal(err)
	}
	err = ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("ValidateConfig() error = %v, want a missing catalog", err)
	}
	// A symlink at the managed path resolves to bytes AIGW may well have written,
	// so the check has to look at the path itself rather than what it points to.
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.WriteFile(elsewhere, []byte(`{"models":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, catalogPath); err != nil {
		t.Skipf("this platform does not allow symlinks here: %v", err)
	}
	err = ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("ValidateConfig() error = %v, want a regular-file report", err)
	}
}

// TestValidateConfigRejectsUnownedManagedCatalogLine catches a marker-bearing
// line the sidecar does not account for, which is the shape a foreign writer or
// a partially reverted projection leaves behind.
func TestValidateConfigRejectsUnownedManagedCatalogLine(t *testing.T) {
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{}, nil, fmt.Errorf("native catalog unavailable")
	}
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(current), "model_catalog_json = ") {
		t.Fatalf("fixture unexpectedly owns a model catalog: %s", current)
	}
	injected := "model_catalog_json = \"/tmp/foreign.json\" # managed by AIGW\n" + string(current)
	if err := os.WriteFile(path, []byte(injected), 0o600); err != nil {
		t.Fatal(err)
	}
	err = ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "does not own") {
		t.Fatalf("ValidateConfig() error = %v, want an ownership report", err)
	}
}
