package cli

import (
	"aigw-cli/internal/clients"
	"aigw-cli/internal/presentation"
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
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

func TestCommandHelpIsProjectedFromCobraMetadata(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Clients: clients.Registry(), Out: out, Err: out}
	command := &cobra.Command{Use: "semantic-root", Short: "source-owned summary"}
	command.AddCommand(
		&cobra.Command{Use: "zeta", Short: "source-owned Z", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "alpha", Short: "source-owned A", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "internal", Hidden: true, Run: func(*cobra.Command, []string) {}},
	)
	command.Flags().String("visible", "", "visible option")
	command.Flags().String("internal", "", "internal option")
	if err := command.Flags().MarkHidden("internal"); err != nil {
		t.Fatal(err)
	}

	renderCommandHelp(app, command)
	help := out.String()
	for _, want := range []string{
		"source-owned summary",
		"semantic-root [command]",
		"alpha",
		"source-owned A",
		"zeta",
		"source-owned Z",
		"Commands",
		"visible option",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help does not project Cobra metadata %q: %q", want, help)
		}
	}
	if strings.Index(help, "alpha") > strings.Index(help, "zeta") {
		t.Fatalf("extension commands are not ordered: %q", help)
	}
	if strings.Contains(help, "internal option") || strings.Contains(help, "internal  ") {
		t.Fatalf("help exposes an internal surface: %q", help)
	}
	originalSorting := cobra.EnableCommandSorting
	cobra.EnableCommandSorting = false
	t.Cleanup(func() { cobra.EnableCommandSorting = originalSorting })
	command.AddCommand(&cobra.Command{Use: "beta", Short: "source-owned B", Run: func(*cobra.Command, []string) {}})
	out.Reset()
	renderCommandHelp(app, command)
	help = out.String()
	if strings.Index(help, "zeta") > strings.Index(help, "beta") {
		t.Fatalf("help overrides Cobra's explicit command ordering: %q", help)
	}
}

func TestCommandHelpIncludesNativeFlagMetadataAndExamples(t *testing.T) {
	for _, columns := range []string{"0", "48"} {
		t.Run(columns, func(t *testing.T) {
			var out bytes.Buffer
			app := &App{Clients: clients.Registry(), Out: &out, Err: &out, Env: []string{"COLUMNS=" + columns}}
			root := &cobra.Command{Use: "service"}
			root.PersistentFlags().String("region", "team", "Choose the service region")
			command := &cobra.Command{
				Use:     "connect",
				Short:   "Connect a service",
				Long:    "Choose a service without changing credentials.\nExisting selections remain available.",
				Example: "service connect --target local\nservice connect --region team",
			}
			root.AddCommand(command)
			command.Flags().String("target", "local", "Use the selected `path`")
			command.Flags().IntP("attempts", "a", 3, "Bound the connection attempts")
			command.Flags().Duration("timeout", 3*time.Second, "Bound the connection duration")
			command.Flags().Bool("verify", true, "Verify the selected endpoint")
			if err := command.Flags().Set("attempts", "7"); err != nil {
				t.Fatal(err)
			}
			renderCommandHelp(app, command)
			text := strings.Join(strings.Fields(out.String()), " ")
			for _, want := range []string{
				command.Long, command.Example, "Examples", "Inherited options",
				command.NonInheritedFlags().FlagUsages(), command.InheritedFlags().FlagUsages(),
			} {
				if !strings.Contains(text, strings.Join(strings.Fields(want), " ")) {
					t.Fatalf("help missing native metadata %q:\n%s", want, &out)
				}
			}
			if columns == "48" {
				for line := range strings.SplitSeq(strings.TrimRight(command.NonInheritedFlags().FlagUsagesWrapped(48), "\n"), "\n") {
					if !strings.Contains(out.String(), "\n"+strings.TrimRight(line, " \t")+"\n") {
						t.Fatalf("help changed native narrow option layout: %q\n%s", line, &out)
					}
				}
				for line := range strings.SplitSeq(out.String(), "\n") {
					if presentation.DisplayWidth(line) > 48 {
						t.Fatalf("help line exceeds available width: %q", line)
					}
				}
			}
			if !strings.Contains(out.String(), "\n  -a, --attempts") || strings.Contains(out.String(), "\n    -a, --attempts") {
				t.Fatalf("option lines do not share the section indentation:\n%s", &out)
			}
			out.Reset()
			command.Example = "service connect --target 'a-long-path-with-two  spaces'\nservice connect --region 'tab\tvalue'"
			renderCommandHelp(app, command)
			want := "  " + strings.ReplaceAll(command.Example, "\n", "\n  ") + "\n"
			if !strings.Contains(out.String(), want) {
				t.Fatalf("help rewrote copyable example bytes:\n%q", out.String())
			}
		})
	}
}
