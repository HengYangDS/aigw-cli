package adapter

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestConflictStatusNamesTheActionThatResolvesARefusal(t *testing.T) {
	cause := errors.New("decoding failed")
	refusal := NewConflictError("settings changed", UndoOutsideEdit("Example"), cause)
	if !errors.Is(refusal, cause) || !refusal.ProjectionConflict() {
		t.Fatalf("conflict = %#v", refusal)
	}
	status, refused := ConflictStatus(fmt.Errorf("planning: %w", refusal))
	want := "Undo the outside edit to the AIGW-managed Example settings, then run `aigw sync`"
	if !refused || status.Ready || status.Issue != "settings changed" || status.RepairAction != want {
		t.Fatalf("status = %#v, %t", status, refused)
	}
	// Any other failure leaves the caller's own diagnosis.
	if status, refused := ConflictStatus(cause); refused || status.RepairAction != "" {
		t.Fatalf("status of another failure = %#v, %t", status, refused)
	}
	if status, refused := ConflictStatus(nil); refused || status.RepairAction != "" {
		t.Fatalf("status without a failure = %#v, %t", status, refused)
	}
}

// A refusal of a deleted client file names only base names, the record that
// remains, and what lets synchronization pass.
func TestMissingWithRecordNamesTheRecordAndNoPrivateFolder(t *testing.T) {
	file, record := filepath.Join("private", "home", "config.yaml"), filepath.Join("private", "home", "config.yaml"+StateSuffix)
	refusal := MissingWithRecord("Example", file, record, " and its neighbor")
	if want := "Example file config.yaml does not exist, but AIGW's record of it, config.yaml.aigw-state.json, remains in its folder"; refusal.Error() != want {
		t.Fatalf("message = %q", refusal.Error())
	}
	if want := "Restore config.yaml, or delete config.yaml.aigw-state.json in its folder and its neighbor if config.yaml is gone for good, then run `aigw sync`"; refusal.NextAction() != want {
		t.Fatalf("next action = %q", refusal.NextAction())
	}
	if strings.Contains(refusal.Error()+refusal.NextAction(), "private") {
		t.Fatalf("the refusal shows a private folder: %q", refusal)
	}
}

// A record that cannot be read is refused by its base name, on one line, with
// the failure as its cause, and its fix restores the record.
func TestUnreadableRecordNamesTheRecordToRestore(t *testing.T) {
	cause := errors.New("invalid record\nunexpected EOF")
	refusal := UnreadableRecord(filepath.Join("private", "home", "config.yaml"+StateSuffix), cause)
	if want := "AIGW's record config.yaml.aigw-state.json cannot be read: invalid record: unexpected EOF"; refusal.Error() != want {
		t.Fatalf("message = %q", refusal.Error())
	}
	if want := "Restore config.yaml.aigw-state.json in its folder from a copy of it, then run `aigw sync`"; refusal.NextAction() != want {
		t.Fatalf("next action = %q", refusal.NextAction())
	}
	if !errors.Is(refusal, cause) || strings.Contains(refusal.Error()+refusal.NextAction(), "private") {
		t.Fatalf("the refusal = %q", refusal)
	}
}

// A file that the user cannot read is named, under home as ~/..., with the step
// that makes it readable; any other failure is not this status.
func TestUnreadableStatusNamesTheFile(t *testing.T) {
	home := filepath.Join(string(filepath.Separator)+"home", "user")
	status, unreadable := UnreadableStatus(fmt.Errorf("read: %w", fs.ErrPermission), filepath.Join(home, ".codex", "config.toml"), home)
	if !unreadable || status.RepairAction != "Make ~/.codex/config.toml and its folder readable and writable, then run `aigw sync`." {
		t.Fatalf("status = %#v, %t", status, unreadable)
	}
	if _, unreadable := UnreadableStatus(fs.ErrNotExist, filepath.Join(home, "x"), home); unreadable {
		t.Fatal("a missing file was taken for an unreadable one")
	}
}

// Before a configuration exists, a refusal names the fix that it carries for
// that state, however the fix is worded, and never `aigw sync`; with one, it
// names its configured fix. A fix that is the same in both states, and any
// other failure, stay as they are.
func TestBeforeConfigurationNamesTheFixThatTheRefusalCarries(t *testing.T) {
	record := filepath.Join("private", "home", "config.yaml"+StateSuffix)
	for _, tc := range []struct {
		refusal *ConflictError
		want    string
	}{
		{NewConflictError("settings changed", UndoOutsideEdit("Example"), nil), "Undo the outside edit to the AIGW-managed Example settings, then run the same command again."},
		{MissingWithRecord("Example", filepath.Join("private", "home", "config.yaml"), record, ""), "Restore config.yaml, or delete config.yaml.aigw-state.json in its folder if config.yaml is gone for good, then run the same command again."},
		{UnreadableRecord(record, errors.New("invalid")), "Restore config.yaml.aigw-state.json in its folder from a copy of it, then run the same command again."},
		// A fix worded otherwise, whose configured step no command can take
		// without a configuration.
		{NewConflictError("profile deleted", Fix{
			Configured:   "Restore profile.json, or run `aigw client disable example` if it is gone for good",
			Unconfigured: "Restore profile.json in its folder from a copy of it, then run the same command again.",
		}, nil), "Restore profile.json in its folder from a copy of it, then run the same command again."},
	} {
		wrapped := fmt.Errorf("setup failed: %w", tc.refusal)
		got := BeforeConfiguration(wrapped)
		next, ok := got.(interface{ NextAction() string })
		if !ok || next.NextAction() != tc.want || strings.Contains(next.NextAction(), "aigw sync") {
			t.Fatalf("next action of %q = %#v", tc.refusal, got)
		}
		if !strings.HasSuffix(tc.refusal.NextAction(), "run `aigw sync`") && !strings.Contains(tc.refusal.NextAction(), "aigw client disable") {
			t.Fatalf("configured next action = %q", tc.refusal.NextAction())
		}
		if conflict, refused := errors.AsType[*ConflictError](got); !refused || conflict != tc.refusal {
			t.Fatalf("the conflict is no longer reachable: %v", got)
		}
		if got.Error() != wrapped.Error() {
			t.Fatalf("message = %q", got)
		}
	}
	same := NewConflictError("syntax", Rerun("Correct the JSON, then run the same command again."), nil)
	got, stated := errors.AsType[interface {
		error
		NextAction() string
	}](BeforeConfiguration(same))
	if !stated || !errors.Is(got, same) || got.NextAction() != "Correct the JSON, then run the same command again." || same.NextAction() != got.NextAction() {
		t.Fatalf("a fix that is the same in both states changed: %v", got)
	}
	plain := errors.New("plain")
	if got := BeforeConfiguration(plain); !errors.Is(got, plain) || got.Error() != "plain" {
		t.Fatal("another failure changed")
	}
}
