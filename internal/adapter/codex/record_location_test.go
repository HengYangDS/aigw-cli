package codex

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/platform"
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

// A record that 0.6.1 kept beside a symbolic link is read in place, and the
// next write moves it beside the resolved file and removes the old one. A
// move whose removal fails restores both, so no record is lost.
func TestAWriteMovesARecordLeftBesideALink(t *testing.T) {
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
	runtime := atomicTestRuntime()
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(alias)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(codexStatePath(resolved), codexStatePath(alias)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(alias, runtime); err != nil {
		t.Fatalf("the record beside the link was not read: %v", err)
	}

	changed := runtime
	changed.UpstreamModel = "gpt-5.6-sol"
	original := removeFileIfUnchanged
	removeFileIfUnchanged = func(path string, before platform.FileSnapshot) (platform.FileSnapshot, error) {
		if path == codexStatePath(alias) {
			return platform.FileSnapshot{}, errors.New("removal failed")
		}
		return original(path, before)
	}
	if _, err := ReconcileConfigs([]TargetRef{HomeTarget(alias)}, []TargetRef{HomeTarget(alias)}, runtime, changed, true); err == nil {
		t.Fatal("a move whose removal failed passed")
	}
	removeFileIfUnchanged = original
	if _, err := os.Stat(codexStatePath(alias)); err != nil {
		t.Fatalf("a failed move lost the record beside the link: %v", err)
	}
	if _, err := os.Stat(codexStatePath(resolved)); !os.IsNotExist(err) {
		t.Fatalf("a failed move left a record beside the file: %v", err)
	}

	// The old record goes only once the new one is in place, so a crash
	// between the two steps leaves a record either way.
	removeFileIfUnchanged = func(path string, before platform.FileSnapshot) (platform.FileSnapshot, error) {
		if path == codexStatePath(alias) {
			if _, err := os.Stat(codexStatePath(resolved)); err != nil {
				t.Errorf("the record beside the link was removed before the new one existed: %v", err)
			}
		}
		return original(path, before)
	}
	t.Cleanup(func() { removeFileIfUnchanged = original })
	if _, err := ReconcileConfigs([]TargetRef{HomeTarget(alias)}, []TargetRef{HomeTarget(alias)}, runtime, changed, true); err != nil {
		t.Fatal(err)
	}
	removeFileIfUnchanged = original
	if _, err := os.Stat(codexStatePath(resolved)); err != nil {
		t.Fatalf("the write did not move the record beside the file: %v", err)
	}
	if _, err := os.Lstat(codexStatePath(alias)); !os.IsNotExist(err) {
		t.Fatalf("the write kept the record beside the link: %v", err)
	}
	if err := ValidateConfig(alias, changed); err != nil {
		t.Fatalf("inspection after the move = %v", err)
	}
}
