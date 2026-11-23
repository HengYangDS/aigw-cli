package codex

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/platform"
)

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

// Codex asks platform.SameFile whether two targets name one file. Two files
// are two targets: converging keeps both, and planning takes neither for a
// duplicate. A file under another name, through a symbolic link or a hard
// link, is one file: converging keeps one, planning refuses both as one file
// named twice, and a change from one name to the other keeps the file.
func TestTargetsAreOneFileOnlyWhenTheSystemSaysSo(t *testing.T) {
	root := t.TempDir()
	first, second := realFile(t, filepath.Join(root, "real", "a.toml")), realFile(t, filepath.Join(root, "real", "b.toml"))
	discovered := discovery.Result{}

	refs, err := targetRefs(discovered, []string{first, second}, "")
	if err != nil {
		t.Fatal(err)
	}
	if targets := convergedTargets(discovered, []string{first, second}); len(targets) != 2 {
		t.Fatalf("two files converged into %q", targets)
	}
	if _, err := normalizeCodexTargets(refs, true); err != nil {
		t.Fatalf("two files were taken for one: %v", err)
	}

	names := map[string]func(link string) error{
		"a symbolic link": func(link string) error { return os.Symlink(first, link) },
		"a hard link":     func(link string) error { return os.Link(first, link) },
	}
	for name, makeLink := range names {
		t.Run(name, func(t *testing.T) {
			other := filepath.Join(t.TempDir(), "other.toml")
			if err := makeLink(other); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("this account cannot create %s: %v", name, err)
				}
				t.Fatal(err)
			}
			if targets := convergedTargets(discovered, []string{other, first}); len(targets) != 1 {
				t.Fatalf("one file converged into %q", targets)
			}
			refs, err := targetRefs(discovered, []string{first, other}, "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := normalizeCodexTargets(refs, true); err == nil {
				t.Fatal("one file named twice was planned as two")
			}
			union, err := codexTargetUnion(refs[1:], refs[:1])
			if err != nil || len(union) != 1 || !union[0].desired {
				t.Fatalf("one file renamed between configurations = %#v, %v", union, err)
			}
		})
	}
}

// A file that does not exist yet, named once through a link to its folder and
// once in the resolved folder, is one file for every decision: Codex's union
// plans it once, and platform.DroppedFiles, which decides for an edit and a
// synchronization which files a change drops, drops nothing between them.
func TestAMissingTargetThroughALinkedFolderIsOneFileForEveryDecision(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "home")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	resolvedFolder, err := filepath.EvalSymlinks(folder)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(folder, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("this account cannot create a symbolic link: %v", err)
		}
		t.Fatal(err)
	}
	through, resolved := filepath.Join(link, "config.toml"), filepath.Join(resolvedFolder, "config.toml")
	refs, err := targetRefs(discovery.Result{}, []string{through, resolved}, "")
	if err != nil {
		t.Fatal(err)
	}
	union, err := codexTargetUnion(refs[:1], refs[1:])
	planned := err == nil && len(union) == 1
	dropped := platform.DroppedFiles([]string{through}, []string{resolved})
	if !planned || len(dropped) != 0 {
		t.Fatalf("one missing file in two spellings: Codex's union plans it once=%t (%d targets, %v), and the change drops %q", planned, len(union), err, dropped)
	}
}
