package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
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

// realFile writes an empty file and returns its path.
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

// linkTo makes a symbolic link to target at link. Windows lets only a
// privileged or developer-mode account create one, so there a test that
// cannot is skipped; everywhere else the failure fails it.
func linkTo(t *testing.T, target, link string) string {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("this account cannot create a symbolic link: %v", err)
		}
		t.Fatal(err)
	}
	return link
}

// On a volume that keeps case, .codex and .Codex are two files: neither names
// the other, and a change from one to the other drops it. On a volume that
// ignores case they are one file, and the change drops nothing.
func TestTwoPathsAreOneFileOnlyWhenTheSystemSaysSo(t *testing.T) {
	home := t.TempDir()
	lower, upper := filepath.Join(home, ".codex", "config.toml"), filepath.Join(home, ".Codex", "config.toml")
	first, second := realFile(t, filepath.Join(home, "real", "a.toml")), realFile(t, filepath.Join(home, "real", "b.toml"))

	onVolume(t, map[string]string{lower: first, upper: second})
	if SameFile(lower, upper) {
		t.Fatal("two files were taken for one")
	}
	if dropped := DroppedFiles([]string{lower}, []string{upper}); !slices.Equal(dropped, []string{lower}) {
		t.Fatalf("a change to the other file drops %q", dropped)
	}

	onVolume(t, map[string]string{lower: first, upper: first})
	if !SameFile(lower, upper) {
		t.Fatal("one file was taken for two")
	}
	if dropped := DroppedFiles([]string{lower}, []string{upper}); len(dropped) != 0 {
		t.Fatalf("a change of the file's spelling drops %q", dropped)
	}
}

// A file is one file through a link to it, through a link to its folder and
// by its resolved path, in either order; another file is not.
func TestAFileThroughALinkIsTheFileItself(t *testing.T) {
	root := t.TempDir()
	file := realFile(t, filepath.Join(root, "home", "config.toml"))
	other := realFile(t, filepath.Join(root, "other", "config.toml"))
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	spellings := map[string]string{
		"a link to the file":   linkTo(t, file, filepath.Join(root, "linked.toml")),
		"a link to its folder": filepath.Join(linkTo(t, filepath.Dir(file), filepath.Join(root, "linked")), "config.toml"),
	}
	for name, spelling := range spellings {
		if !SameFile(spelling, resolved) || !SameFile(resolved, spelling) {
			t.Errorf("%s and the resolved path were taken for two files", name)
		}
		for _, change := range [][2][]string{{{spelling}, {resolved}}, {{resolved}, {spelling}}} {
			if dropped := DroppedFiles(change[0], change[1]); len(dropped) != 0 {
				t.Errorf("%s: a change from %q to %q drops %q", name, change[0], change[1], dropped)
			}
		}
		if dropped := DroppedFiles([]string{spelling, other}, []string{resolved}); !slices.Equal(dropped, []string{other}) {
			t.Errorf("%s: a change that keeps the file and leaves another drops %q", name, dropped)
		}
	}
}

// The canonical form is absolute, resolves a link to the file, and is the same
// before a file under a linked folder exists and after.
func TestCanonicalPathResolvesLinksAndIsStableBeforeCreation(t *testing.T) {
	root := t.TempDir()
	file := realFile(t, filepath.Join(root, "real", "target.toml"))
	resolved, err := CanonicalPath(file)
	if err != nil || !filepath.IsAbs(resolved) {
		t.Fatalf("the canonical form of a file = %q, %v; want an absolute path", resolved, err)
	}
	if got, err := CanonicalPath(linkTo(t, file, filepath.Join(root, "link.toml"))); err != nil || got != resolved {
		t.Errorf("the canonical form of a link to the file = %q, %v; want %q", got, err, resolved)
	}
	target := filepath.Join(linkTo(t, filepath.Dir(file), filepath.Join(root, "alias")), "new", "config.toml")
	before, err := CanonicalPath(target)
	if err != nil {
		t.Fatal(err)
	}
	realFile(t, target)
	if after, err := CanonicalPath(target); err != nil || after != before {
		t.Errorf("the canonical form changed when the file was created: before %q, after %q, %v", before, after, err)
	}
}

// A file in the way refuses the canonical form, classified at the path as
// given, and a link that leads to itself is refused as a failure to resolve
// links. Neither path names a file that another path could be.
func TestCanonicalPathRefusesAPathItCannotResolve(t *testing.T) {
	root := t.TempDir()
	through := filepath.Join(realFile(t, filepath.Join(root, "file")), "config.toml")
	_, err := CanonicalPath(through)
	if failure, classified := errors.AsType[*PathConditionError](err); !classified || !failure.Blocked() || failure.Path != through {
		t.Errorf("a path through a file = %v; want it blocked at %s", err, through)
	}
	if SameFile(through, filepath.Join(root, "other", "config.toml")) {
		t.Error("a path through a file was taken for another path's file")
	}
	loop := linkTo(t, filepath.Join(root, "loop"), filepath.Join(root, "loop"))
	if _, err := CanonicalPath(loop); err == nil || !strings.Contains(err.Error(), "resolve symbolic links") {
		t.Errorf("a link to itself = %v; want a failure to resolve symbolic links", err)
	}
}

// A path that names no existing file is the same file as another when their
// canonical forms are equal: the links of the part of each path that exists
// are resolved, and the names of the parts that do not exist yet are kept. So
// a missing file through a link to its folder is the file at the resolved
// folder, as it will be once it exists, and a missing file of another name is
// another file.
func TestAMissingFileIsOneFileByItsCanonicalForm(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "home")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(folder, "config.toml")
	unclean := strings.Join([]string{folder, ".", "nested", "..", "config.toml"}, string(filepath.Separator))
	if !SameFile(missing, unclean) {
		t.Error("two spellings that clean to one path were taken for two files")
	}
	through := filepath.Join(linkTo(t, folder, filepath.Join(root, "linked")), "config.toml")
	if !SameFile(through, missing) || !SameFile(missing, through) {
		t.Error("a missing file through a folder link was taken for another file than the one in the resolved folder")
	}
	if dropped := DroppedFiles([]string{through}, []string{missing}); len(dropped) != 0 {
		t.Errorf("a change between two spellings of a missing file drops %q", dropped)
	}
	other := filepath.Join(folder, "other.toml")
	if dropped := DroppedFiles([]string{through}, []string{other}); !slices.Equal(dropped, []string{through}) {
		t.Errorf("a change to a missing file of another name drops %q", dropped)
	}
}
