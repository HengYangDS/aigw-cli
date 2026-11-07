package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A guarded write, removal or restore that fails names its file and keeps its
// own text; one that meets a file changed since it was read says so.
func TestGuardedFileOperationsNameTheirFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("current\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := NewFileSnapshot([]byte("read earlier\n"), 0o600)
	_, err := WriteFileAtomicIfUnchanged(path, stale, []byte("new\n"), 0o600)
	failed, ok := errors.AsType[*FileError](err)
	if !ok || failed.Path != path || !failed.Changed || !strings.HasPrefix(err.Error(), "preimage changed for "+path) {
		t.Fatalf("a write over a changed file = %#v, %v", failed, err)
	}
	_, err = RemoveFileIfUnchanged(path, stale)
	if failed, ok := errors.AsType[*FileError](err); !ok || failed.Path != path || !failed.Changed {
		t.Fatalf("a removal of a changed file = %v", err)
	}
	err = RestoreFileAtomicIfPostimage(path, FileSnapshot{}, stale)
	if failed, ok := errors.AsType[*FileError](err); !ok || failed.Path != path || !failed.Changed || !strings.HasPrefix(err.Error(), "postimage changed for "+path) {
		t.Fatalf("a restore over a changed file = %v", err)
	}
	if err := RestoreFileAtomicIfPostimage(path, FileSnapshot{}, FileSnapshot{}); err == nil {
		t.Fatal("a restore of an absent file over an existing one succeeded")
	}
}

// A failed write reports what its undo did to each file, starts with what an
// inner attempt undid, and is an incomplete compensation only when one of its
// own restores failed.
func TestFailedWriteReportsEachRestoredFile(t *testing.T) {
	cause := errors.New("write failed")
	if _, reported := errors.AsType[*WriteError](FailedWrite(cause, nil)); reported {
		t.Fatal("a write that undid nothing reports restored files")
	}
	inner := FailedWrite(cause, Restoration{{Path: "/inner"}})
	restoreFailure := &FileError{Path: "/outer", Err: os.ErrPermission}
	err := FailedWrite(inner, Restoration{{Path: "/outer", Err: restoreFailure}})
	failure, ok := errors.AsType[*WriteError](err)
	if !ok || len(failure.Restored) != 2 || failure.Restored[0].Path != "/inner" || failure.Restored[1].Path != "/outer" {
		t.Fatalf("nested failure = %#v", failure)
	}
	if !errors.Is(err, ErrCompensationIncomplete) || !errors.Is(err, cause) || !strings.Contains(err.Error(), "restore /outer: permission denied") {
		t.Fatalf("an incomplete compensation = %v", err)
	}
	if errors.Is(inner, ErrCompensationIncomplete) {
		t.Fatalf("a complete undo is marked incomplete: %v", inner)
	}
	pathless := Restoration{{Err: errors.New("rollback failed")}}
	if got := pathless.Err().Error(); got != "rollback failed" {
		t.Fatalf("a failure without a file = %q", got)
	}
}
