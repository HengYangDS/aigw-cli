package cli_test

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
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
		configuration.ClientClaude: executableFixture(t, configuration.ClientClaude),
	}}}
	for _, step := range []struct {
		args    []string
		stdin   string
		prepare func(t *testing.T, app *cli.App)
	}{
		{args: []string{"setup", "--account", "gateway", "--route", "claude-team", "--label", "Gateway",
			"--anthropic-url", "https://gateway.test", "--for", "claude", "--model", "claude-team", "--token-stdin"}, stdin: "fixture-token\n"},
		{args: []string{"sync"}},
		{args: []string{"use", "--for", "claude", "claude-next"}, prepare: addNextClaudeRoute},
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
			if want := map[string]int{base.Executable: 1}; !maps.Equal(others, want) || files.opens[reader] > 1 {
				t.Fatalf("%s opened executables %v; want the program once and the reader at most once", step.args[0], files.opens)
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
	addAccountRoute(&cfg, "claude-next", "gateway", "Gateway", configuration.Endpoints{Anthropic: "https://gateway.test"}, configuration.ClientClaude, "claude-next")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}
