package codex

import (
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/discovery"
)

// onVolume makes each path in names read the identity of its real file, so a
// test stands for a volume that keeps case, where two names that differ only
// in case are two files, or one that ignores case, where they are one.
func onVolume(t *testing.T, names map[string]string) {
	t.Helper()
	original := statTarget
	t.Cleanup(func() { statTarget = original })
	statTarget = func(path string) (os.FileInfo, error) {
		if file, named := names[filepath.Clean(path)]; named {
			return os.Stat(file)
		}
		return original(path)
	}
}

// realFile writes a file and returns its path.
func realFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// On a volume that keeps case, .codex and .Codex are two files: converging
// keeps both, and planning takes neither for a duplicate. On a volume that
// ignores case they are one file: converging keeps one, and planning refuses
// both as one file named twice.
func TestTargetsAreOneFileOnlyWhenTheSystemSaysSo(t *testing.T) {
	home := t.TempDir()
	lower, upper := filepath.Join(home, ".codex", "config.toml"), filepath.Join(home, ".Codex", "config.toml")
	discovered := discovery.Result{Surfaces: []discovery.Surface{{ID: string(HomeDefault), ConfigPath: lower, AutoManaged: true}}}
	first, second := realFile(t, filepath.Join(home, "real", "a.toml")), realFile(t, filepath.Join(home, "real", "b.toml"))

	onVolume(t, map[string]string{lower: first, upper: second})
	for _, current := range [][]string{{upper}, {lower, upper}} {
		if targets := convergedTargets(discovered, current); len(targets) != 2 {
			t.Fatalf("two files converged from %q into %q", current, targets)
		}
	}
	refs, err := targetRefs(discovered, []string{lower, upper}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeCodexTargets(refs, true); err != nil {
		t.Fatalf("two files were taken for one: %v", err)
	}

	onVolume(t, map[string]string{lower: first, upper: first})
	if targets := convergedTargets(discovered, []string{upper}); len(targets) != 1 {
		t.Fatalf("one file converged into %q", targets)
	}
	if _, err := normalizeCodexTargets(refs, true); err == nil {
		t.Fatal("one file named twice was planned as two")
	}
	union, err := codexTargetUnion(refs[1:], refs[:1])
	if err != nil || len(union) != 1 || !union[0].desired {
		t.Fatalf("one file renamed between configurations = %#v, %v", union, err)
	}
}
