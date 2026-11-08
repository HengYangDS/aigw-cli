package adapter

import (
	"errors"
	"fmt"
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
