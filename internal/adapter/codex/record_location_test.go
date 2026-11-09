package codex

import (
	"os"
	"path/filepath"
	"testing"
)

// A target through a symbolic link keeps AIGW's record beside the resolved
// file, so the projection and its inspection read one record, and a target
// named both ways shares it.
func TestASymlinkedTargetReadsTheRecordThatItsProjectionWrote(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "work", "codex.toml")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	writeCodexFixture(t, file, "model = \"gpt-native\"\n")
	alias := filepath.Join(dir, "alias.toml")
	if err := os.Symlink(file, alias); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := codexRecordPath(alias), codexStatePath(resolved); got != want {
		t.Fatalf("the record of the alias = %s, want %s", got, want)
	}
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(alias)}, atomicTestRuntime(), atomicTestRuntime(), false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(alias, atomicTestRuntime()); err != nil {
		t.Fatalf("inspecting the alias after its projection = %v", err)
	}
	if err := ValidateConfig(file, atomicTestRuntime()); err != nil {
		t.Fatalf("inspecting the file file after the alias's projection = %v", err)
	}
	if _, err := os.Lstat(codexStatePath(alias)); !os.IsNotExist(err) {
		t.Fatalf("a second record beside the link: %v", err)
	}
}
