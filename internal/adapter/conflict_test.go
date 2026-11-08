package adapter

import (
	"errors"
	"fmt"
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
