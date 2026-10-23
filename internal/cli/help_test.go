package cli

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestRootHelpKeepsCommandGroupsInColumns(t *testing.T) {
	for _, width := range []int{0, 80, 100} {
		var out bytes.Buffer
		app := &App{Out: &out, Err: &out, Env: []string{"COLUMNS=" + strconv.Itoa(width)}}
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
