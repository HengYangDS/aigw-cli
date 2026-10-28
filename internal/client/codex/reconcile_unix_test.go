//go:build !windows

package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileConfigsCreatesDefaultTargetOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".codex", "config.toml")
	target := codexHomeTarget(path)
	target.CreateIfAbsent = true

	if _, err := ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("created config mode = %s, want 0600", info.Mode().Perm())
	}
}
