package cli

import (
	"aigw-cli/internal/clients"
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestRootHelpKeepsCommandGroupsInColumns(t *testing.T) {
	for _, width := range []int{0, 80, 100} {
		var out bytes.Buffer
		app := &App{Clients: clients.Registry(), Out: &out, Err: &out, Env: []string{"COLUMNS=" + strconv.Itoa(width)}}
		root := NewRoot(app)
		renderCommandHelp(app, root)
		lines := strings.Split(out.String(), "\n")
		for _, child := range root.Commands() {
			if !child.IsAvailableCommand() {
				continue
			}
			inColumns := false
			for _, line := range lines {
				if rest, found := strings.CutPrefix(line, "  "+child.Name()+"  "); found && strings.HasPrefix(strings.TrimLeft(rest, " "), strings.Fields(child.Short)[0]) {
					inColumns = true
					break
				}
			}
			if !inColumns {
				t.Fatalf("width=%d: %s left its description column:\n%s", width, child.Name(), out.String())
			}
		}
	}
}

// Root help starts with commands to copy. Each keeps one logical line at every
// width; only its description wraps.
func TestRootHelpKeepsStartCommandsWhole(t *testing.T) {
	for _, width := range []int{0, 24, 32, 48, 100} {
		var out bytes.Buffer
		app := &App{Clients: clients.Registry(), Out: &out, Err: &out, Env: []string{"COLUMNS=" + strconv.Itoa(width)}}
		renderCommandHelp(app, NewRoot(app))
		lines := strings.Split(out.String(), "\n")
		for _, command := range []string{"aigw setup", "aigw use --for <client> <route>", "aigw check"} {
			whole := false
			for _, line := range lines {
				whole = whole || line == "  "+command || strings.HasPrefix(line, "  "+command+"  ")
			}
			if !whole {
				t.Fatalf("width=%d split the command %q:\n%s", width, command, out.String())
			}
		}
	}
}
