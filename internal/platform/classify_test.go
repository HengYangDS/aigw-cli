package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// classifyFixture is a folder that holds each kind of entry that a path can
// run through: a regular file, a folder, a link to each, and a link that leads
// nowhere. links reports whether the host could create the links.
type classifyFixture struct {
	root, file, folder, toFolder, toFile, dangling string
	links                                          bool
}

func newClassifyFixture(t *testing.T) classifyFixture {
	t.Helper()
	root := t.TempDir()
	fixture := classifyFixture{
		root: root, file: filepath.Join(root, "file"), folder: filepath.Join(root, "folder"),
		toFolder: filepath.Join(root, "to-folder"), toFile: filepath.Join(root, "to-file"), dangling: filepath.Join(root, "dangling"),
	}
	if err := os.WriteFile(fixture.file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.folder, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture.links = os.Symlink(fixture.folder, fixture.toFolder) == nil &&
		os.Symlink(fixture.file, fixture.toFile) == nil &&
		os.Symlink(filepath.Join(root, "nowhere"), fixture.dangling) == nil
	return fixture
}

// failed is the error of an operation at path: Windows reports a path through
// a regular file as missing, so each case may pass that shape too.
func failed(op, path string, err error) error { return &fs.PathError{Op: op, Path: path, Err: err} }

// Every condition is decided by the file system and the kind of path that the
// operation expected, never by the error's number alone: a path through a
// regular file is in that file's way whether the error reads ENOTDIR or, as on
// Windows, a missing path, and the path itself being a regular file is never
// in the way of an operation that expects a file.
func TestClassifyPathDecidesOneCondition(t *testing.T) {
	f := newClassifyFixture(t)
	under := func(parts ...string) string { return filepath.Join(append([]string{f.root}, parts...)...) }
	for _, test := range []struct {
		name     string
		path     string
		kind     PathKind
		err      error
		want     PathCondition
		culprit  string
		needLink bool
	}{
		{name: "the file itself, unreadable", path: f.file, kind: FileExpected, err: failed("open", f.file, fs.ErrPermission), want: PathPermission},
		{name: "a file in the way, as POSIX reports it", path: under("file", "config.toml"), err: failed("open", under("file", "config.toml"), syscall.ENOTDIR), want: PathFileInTheWay, culprit: f.file},
		{name: "a file in the way, as Windows reports it", path: under("file", "config.toml"), err: failed("open", under("file", "config.toml"), fs.ErrNotExist), want: PathFileInTheWay, culprit: f.file},
		{name: "a file in the way higher up", path: under("file", "a", "b", "config.toml"), err: failed("open", under("file", "a", "b", "config.toml"), fs.ErrNotExist), want: PathFileInTheWay, culprit: f.file},
		{name: "a folder path that is a file", path: f.file, kind: FolderExpected, err: failed("mkdir", f.file, syscall.ENOTDIR), want: PathFileInTheWay, culprit: f.file},
		{name: "a folder path under a file", path: under("file", "sub"), kind: FolderExpected, err: failed("mkdir", f.file, syscall.ENOTDIR), want: PathFileInTheWay, culprit: f.file},
		{name: "a missing file in a folder", path: under("folder", "config.toml"), err: failed("open", under("folder", "config.toml"), fs.ErrNotExist), want: PathMissingFile},
		{name: "a missing folder", path: under("folder", "gone", "config.toml"), err: failed("open", under("folder", "gone", "config.toml"), fs.ErrNotExist), want: PathMissingFolder},
		{name: "a missing folder path", path: under("folder", "gone"), kind: FolderExpected, err: failed("stat", under("folder", "gone"), fs.ErrNotExist), want: PathMissingFolder},
		{name: "permission with nothing in the way", path: under("folder", "config.toml"), err: failed("open", under("folder", "config.toml"), fs.ErrPermission), want: PathPermission},
		{name: "another error", path: under("folder", "config.toml"), err: failed("read", under("folder", "config.toml"), syscall.EIO), want: PathOther},
		{name: "an existing name that no link explains", path: under("folder", "config.toml"), err: failed("open", under("folder", "config.toml"), fs.ErrExist), want: PathOther},
		{name: "no error", path: under("folder", "config.toml"), err: nil, want: PathOther},
		{name: "a link that leads nowhere above a file", path: under("dangling", "config.toml"), err: failed("open", under("dangling", "config.toml"), fs.ErrNotExist), want: PathDanglingLink, culprit: f.dangling, needLink: true},
		{name: "a file that is a link leading nowhere", path: f.dangling, err: failed("open", f.dangling, fs.ErrNotExist), want: PathDanglingLink, culprit: f.dangling, needLink: true},
		{name: "a folder path that is a link leading nowhere", path: f.dangling, kind: FolderExpected, err: failed("mkdir", f.dangling, fs.ErrExist), want: PathDanglingLink, culprit: f.dangling, needLink: true},
		{name: "a link to a file where a folder is needed", path: under("to-file", "config.toml"), err: failed("open", under("to-file", "config.toml"), syscall.ENOTDIR), want: PathDanglingLink, culprit: f.toFile, needLink: true},
		{name: "a file that is a link to a file", path: f.toFile, err: failed("open", f.toFile, fs.ErrPermission), want: PathPermission, needLink: true},
		{name: "a missing file through a link to a folder", path: under("to-folder", "config.toml"), err: failed("open", under("to-folder", "config.toml"), fs.ErrNotExist), want: PathMissingFile, needLink: true},
		{name: "a missing folder through a link to a folder", path: under("to-folder", "gone", "config.toml"), err: failed("open", under("to-folder", "gone", "config.toml"), fs.ErrNotExist), want: PathMissingFolder, needLink: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.needLink && !f.links {
				t.Skip("symbolic links are unavailable")
			}
			failure := ClassifyPath(test.path, test.kind, test.err)
			// The failure keeps the error that the operation returned, itself.
			if failure.Condition != test.want || failure.Culprit != test.culprit || failure.Path != test.path || !errors.Is(failure.Err, test.err) {
				t.Fatalf("ClassifyPath = %+v, want condition %d, culprit %q", failure, test.want, test.culprit)
			}
		})
	}
}

// A relative path is classified where the process runs, and its culprit stays
// relative, as the operator gave it.
func TestClassifyPathKeepsARelativePathRelative(t *testing.T) {
	f := newClassifyFixture(t)
	t.Chdir(f.root)
	path := filepath.Join("file", "config.toml")
	if failure := ClassifyPath(path, FileExpected, failed("open", path, fs.ErrNotExist)); failure.Condition != PathFileInTheWay || failure.Culprit != "file" {
		t.Fatalf("a relative path through a file = %+v", failure)
	}
	path = filepath.Join("folder", "config.toml")
	if failure := ClassifyPath(path, FileExpected, failed("open", path, fs.ErrNotExist)); failure.Condition != PathMissingFile {
		t.Fatalf("a relative missing file = %+v", failure)
	}
}

// A file in the way reads as not a directory on every platform, never as the
// missing path that Windows reports; a link that leads nowhere reads as
// neither missing nor existing; a missing path and a permission error keep
// their own errors.
func TestAPathConditionErrorReadsAsItsCondition(t *testing.T) {
	f := newClassifyFixture(t)
	path := filepath.Join(f.file, "config.toml")
	blocked := ClassifyPath(path, FileExpected, failed("open", path, fs.ErrNotExist))
	if !errors.Is(blocked, ErrFileAsFolder) || !errors.Is(blocked, syscall.ENOTDIR) || errors.Is(blocked, fs.ErrNotExist) ||
		blocked.Reason() != "not a directory" || blocked.Error() != "open "+path+": not a directory" || !blocked.Blocked() || blocked.Absent() {
		t.Fatalf("a file in the way = %v, %+v", blocked, blocked)
	}
	missing := filepath.Join(f.folder, "config.toml")
	absent := ClassifyPath(missing, FileExpected, failed("open", missing, fs.ErrNotExist))
	if !errors.Is(absent, fs.ErrNotExist) || !absent.Absent() || absent.Blocked() || absent.Reason() != "no such file or directory" {
		t.Fatalf("a missing file = %v, %+v", absent, absent)
	}
	denied := ClassifyPath(missing, FileExpected, failed("open", missing, fs.ErrPermission))
	if !errors.Is(denied, fs.ErrPermission) || denied.Reason() != "permission denied" || denied.Error() != "open "+missing+": permission denied" {
		t.Fatalf("a permission error = %v, %+v", denied, denied)
	}
	if !f.links {
		return
	}
	toFile := filepath.Join(f.toFile, "config.toml")
	if failure := ClassifyPath(toFile, FileExpected, failed("open", toFile, syscall.ENOTDIR)); failure.Condition != PathDanglingLink ||
		failure.Reason() != "not a directory" || !errors.Is(failure, ErrFileAsFolder) {
		t.Fatalf("a link to a file where a folder is needed = %v, %+v", failure, failure)
	}
	link := filepath.Join(f.dangling, "config.toml")
	dangling := ClassifyPath(link, FileExpected, failed("open", link, fs.ErrNotExist))
	if errors.Is(dangling, fs.ErrNotExist) || errors.Is(dangling, fs.ErrExist) || !dangling.Blocked() || dangling.Reason() != "broken symbolic link" {
		t.Fatalf("a link that leads nowhere = %v, %+v", dangling, dangling)
	}
	// An operation that classified its failure keeps the classification, which
	// a dangling link, unwrapping to nothing, could not give again.
	if again := ClassifyPath(link, FileExpected, dangling); again != dangling {
		t.Fatalf("a classified failure was classified again: %+v", again)
	}
}

// A file path that a file in the way or a link that leads nowhere blocks is
// refused before anything acts on it; a missing file or one that is there is
// not.
func TestRefuseBlockedFileRefusesOnlyABlockedPath(t *testing.T) {
	f := newClassifyFixture(t)
	refusal, refused := errors.AsType[*PathConditionError](RefuseBlockedFile(filepath.Join(f.file, "config.toml")))
	if !refused || refusal.Condition != PathFileInTheWay || refusal.Culprit != f.file {
		t.Fatalf("a file under a file = %+v", refusal)
	}
	for _, path := range []string{f.file, filepath.Join(f.folder, "config.toml"), filepath.Join(f.folder, "gone", "config.toml")} {
		if err := RefuseBlockedFile(path); err != nil {
			t.Fatalf("%s was refused: %v", path, err)
		}
	}
	if f.links {
		if refusal, refused := errors.AsType[*PathConditionError](RefuseBlockedFile(filepath.Join(f.dangling, "config.toml"))); !refused || refusal.Condition != PathDanglingLink {
			t.Fatalf("a file under a link that leads nowhere = %+v", refusal)
		}
	}
}
