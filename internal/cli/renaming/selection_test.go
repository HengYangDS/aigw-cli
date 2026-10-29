package renaming

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/store"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type selectionPrompt struct {
	selected  string
	selectErr error
	text      string
	textErr   error
}

func (prompt selectionPrompt) Secret(string) (string, error) { return "", nil }
func (prompt selectionPrompt) Select(string, []prompt.Choice) (string, error) {
	return prompt.selected, prompt.selectErr
}
func (prompt selectionPrompt) Text(string) (string, error) { return prompt.text, prompt.textErr }

func TestResolveRenameIDsErrorBranches(t *testing.T) {
	if err := NewRouteCommand(invocation.Context{Clients: clients.Registry()}).ValidateArgs(nil); err == nil {
		t.Fatal("expected non-interactive error")
	}
	if err := NewRouteCommand(invocation.Context{Clients: clients.Registry(), Interactive: true}).ValidateArgs(nil); err == nil {
		t.Fatal("expected missing prompt error")
	}
	if _, _, err := resolveIDs(invocation.Context{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(t.TempDir(), "empty.toml"), clients.Admission()), Interactive: true, Prompt: selectionPrompt{}}, "route", nil); err == nil || !strings.Contains(err.Error(), "No routes") {
		t.Fatalf("error = %v", err)
	}
	want := errors.New("select failed")
	deps := renameRuntime(t)
	deps.Interactive = true
	deps.Prompt = selectionPrompt{selectErr: want}
	if _, _, err := resolveIDs(deps, "route", nil); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	want = errors.New("text failed")
	deps.Prompt = selectionPrompt{selected: "old", textErr: want}
	if _, _, err := resolveIDs(deps, "route", nil); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
