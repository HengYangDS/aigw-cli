package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/store"
)

// TestASecondProtocolNamesItsCommandsHelp gives each command that takes one
// --protocol a second one: flag parsing refuses it, before any configuration
// access, and the failure names that command's own help.
func TestASecondProtocolNamesItsCommandsHelp(t *testing.T) {
	for _, path := range [][]string{
		{"use"}, {"setup"},
		{"model", "list"}, {"model", "show"}, {"model", "add"}, {"model", "remove"},
	} {
		var out bytes.Buffer
		configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration", "aigw.toml"), clients.Admission())
		app := &App{Clients: clients.Registry(), Config: configStore, Out: &out, Err: io.Discard}
		args := append(append([]string{}, path...), "--protocol", "chat", "--protocol", "anthropic")
		if err := Execute(app, args); err == nil {
			t.Fatalf("aigw %s accepted a second protocol", strings.Join(path, " "))
		}
		text := strings.Join(strings.Fields(out.String()), " ")
		help := "aigw " + strings.Join(path, " ") + " --help"
		if !strings.Contains(text, "this flag takes one protocol") || !strings.Contains(text, "Fix "+help) {
			t.Fatalf("aigw %s with a second protocol:\n%s", strings.Join(path, " "), out.String())
		}
	}
}
