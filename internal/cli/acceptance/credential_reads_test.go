package cli_test

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// countingFiles counts the opens of each path. It serves the program from
// another location, so a read that bypasses this capability cannot find it.
type countingFiles struct {
	mutex    sync.Mutex
	opens    map[string]int
	redirect map[string]string
}

func (files *countingFiles) Open(name string) (*os.File, error) {
	files.mutex.Lock()
	files.opens[name]++
	files.mutex.Unlock()
	if path, ok := files.redirect[name]; ok {
		return os.Open(path)
	}
	return os.Open(name)
}

func TestReaderCommandsReadEachAIGWExecutableOnce(t *testing.T) {
	base, _, _, _, _ := testApp(t, "")
	root := t.TempDir()
	base.Executable = filepath.Join(root, "bin", executableName("aigw"))
	base.DataDir = filepath.Join(root, "data")
	program := filepath.Join(root, "program", executableName("aigw"))
	writeFile(t, program, []byte("portable program"), 0o755)
	base.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{
		claude.ID: executableFixture(t, claude.ID),
	}}}
	for _, step := range []struct {
		args    []string
		stdin   string
		prepare func(t *testing.T, app *cli.App)
		// programReads counts identity plus, when the reader is installed, its copy.
		programReads int
	}{
		{args: []string{"setup", "--account", "gateway", "--route", "claude-team", "--label", "Gateway",
			"--anthropic-url", "https://gateway.test", "--for", "claude", "--upstream-model", "claude-team", "--token-stdin"}, stdin: "fixture-token\n", programReads: 2},
		{args: []string{"sync"}, programReads: 1},
		{args: []string{"use", "--for", "claude", "claude-next"}, prepare: addNextClaudeRoute, programReads: 1},
	} {
		t.Run(step.args[0], func(t *testing.T) {
			if step.prepare != nil {
				step.prepare(t, base)
			}
			// Each command runs in its own App, as each production invocation does.
			app := *base
			app.In = strings.NewReader(step.stdin)
			files := &countingFiles{opens: map[string]int{}, redirect: map[string]string{base.Executable: program}}
			app.UseExecutableIdentity(files, executableName("aigw"))
			if err := cli.Execute(&app, step.args); err != nil {
				t.Fatalf("%s: %v", step.args[0], err)
			}
			reader, err := app.ResolveCredentialPath()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(reader); err != nil {
				t.Fatalf("%s left no verified reader: %v", step.args[0], err)
			}
			others := maps.Clone(files.opens)
			delete(others, reader)
			// The reader is verified once before projection and once after it.
			if want := map[string]int{base.Executable: step.programReads}; !maps.Equal(others, want) || files.opens[reader] != 2 {
				t.Fatalf("%s opened executables %v; want the program %d times and the reader twice", step.args[0], files.opens, step.programReads)
			}
		})
	}
}

func addNextClaudeRoute(t *testing.T, app *cli.App) {
	t.Helper()
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	addAccountRoute(&cfg, "claude-next", "gateway", "Gateway", domain.Endpoints{Anthropic: "https://gateway.test"}, claude.ID, "claude-next")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}
