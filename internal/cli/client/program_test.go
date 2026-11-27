package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/platform/processtest"
	"aigw-cli/internal/terminal"
)

// installedClaudeExtension puts one installed version of the Claude Code
// extension into a new extensions directory and returns the directory and the
// version's program.
func installedClaudeExtension(t *testing.T) (string, string) {
	t.Helper()
	extensions := t.TempDir()
	program := filepath.Join(extensions, "anthropic.claude-code-2.1.0", "resources", "native-binary", "claude")
	processtest.Write(t, program, processtest.Failing)
	return extensions, program
}

// Enable without a declaration adopts the program that discovery finds and
// saves it; when discovery finds none, it saves the client enabled, writes
// none of its files and names the repair, and its preview saves nothing.
func TestEnableWithoutADeclarationAdoptsTheDiscoveredProgram(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		runtime, _, secretStore, _ := adapterRuntime(t, adapterConfig())
		if err := secretStore.Set("gateway", "token"); err != nil {
			t.Fatal(err)
		}
		found := processtest.Program(t, "/opt/claude")
		runtime.Discovery = adapterDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: found}}}
		if err := executeAdapter(t, runtime, "enable", claude.ID); err != nil {
			t.Fatal(err)
		}
		saved, err := runtime.Config.Load()
		if binding := saved.Clients[claude.ID]; err != nil || !binding.Enabled || binding.Program != found || binding.Extensions != "" {
			t.Fatalf("saved Claude Code = %#v, %v; want the discovered program", binding, err)
		}
		if _, err := os.Stat(claudeSettingsPath(t, runtime.Host)); err != nil {
			t.Fatalf("Claude Code's settings were not written: %v", err)
		}
	})
	for _, dryRun := range []bool{false, true} {
		name := "not found"
		if dryRun {
			name += " in a preview"
		}
		t.Run(name, func(t *testing.T) {
			runtime, _, secretStore, _ := adapterRuntime(t, adapterConfig())
			if err := secretStore.Set("gateway", "token"); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(runtime.Config.Path())
			if err != nil {
				t.Fatal(err)
			}
			args, impact := []string{"enable", claude.ID}, "AIGW saved Claude Code as enabled and wrote none of its files."
			if dryRun {
				args, impact = append(args, "--dry-run"), "A run would save Claude Code as enabled and write none of its files."
			}
			problem := terminal.ProblemFor(executeAdapter(t, runtime, args...))
			if problem.Title != "Claude Code program was not found" || problem.Impact != impact ||
				problem.Fix != adapter.ProgramNotFound(adapterConfig(), claude.ID).NextAction() ||
				!strings.Contains(problem.Fix, "aigw client edit claude --program <path> or --extensions <dir>") {
				t.Fatalf("enable without a program = %+v", problem)
			}
			if _, err := os.Stat(claudeSettingsPath(t, runtime.Host)); !os.IsNotExist(err) {
				t.Fatalf("Claude Code's settings were written: %v", err)
			}
			saved, err := runtime.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if after, _ := os.ReadFile(runtime.Config.Path()); dryRun && string(after) != string(before) {
				t.Fatal("the preview saved the configuration")
			}
			if binding := saved.Clients[claude.ID]; binding.Enabled == dryRun || binding.DeclaresProgram() {
				t.Fatalf("saved Claude Code = %#v", binding)
			}
		})
	}
}

// A client that declares an extensions directory keeps the directory, never
// the program that it resolves to, and a declaration through edit replaces
// the other.
func TestAnExtensionsDeclarationKeepsTheDirectoryAndReplacesTheProgram(t *testing.T) {
	runtime, out, secretStore, _ := adapterRuntime(t, adapterConfig())
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	extensions, _ := installedClaudeExtension(t)
	if err := executeAdapter(t, runtime, "enable", claude.ID, "--extensions", extensions); err != nil {
		t.Fatal(err)
	}
	saved, err := runtime.Config.Load()
	if binding := saved.Clients[claude.ID]; err != nil || binding.Program != "" || binding.Extensions != extensions {
		t.Fatalf("saved Claude Code = %#v, %v; want only the extensions directory", binding, err)
	}
	program := processtest.Program(t, "/opt/claude")
	if document := edited(t, runtime, out, claude.ID, "--program", program); document.Program != program || document.Extensions != "" {
		t.Fatalf("edit to a program = %+v", document)
	}
	if document := edited(t, runtime, out, claude.ID, "--extensions", extensions); document.Program != "" || document.Extensions != extensions {
		t.Fatalf("edit to an extensions directory = %+v", document)
	}
	// Edit refuses as enable does, naming edit in its fix.
	_, versioned := installedClaudeExtension(t)
	err = executeAdapter(t, runtime, "edit", claude.ID, "--program", versioned)
	if fix := terminal.ProblemFor(err).Fix; !strings.HasPrefix(fix, "aigw client edit claude --extensions ") {
		t.Fatalf("edit to a versioned program = %v, fix %q", err, fix)
	}
}

// The list names a declared extensions directory beside the program that it
// resolves to; a directory that resolves to none is listed with no program
// and not installed.
func TestListNamesTheExtensionsDirectoryBesideItsProgram(t *testing.T) {
	runtime, out := listRuntime(t)
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	extensions, program := installedClaudeExtension(t)
	empty := t.TempDir()
	cfg.DeclareProgram(claude.ID, "", extensions)
	cfg.DeclareProgram(codex.ID, "", empty)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "list", "--json"); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Clients []clientItem `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	byID := map[string]clientItem{}
	for _, item := range document.Clients {
		byID[item.ID] = item
	}
	if got := byID[claude.ID]; !got.Installed || got.Program != program || got.Extensions != extensions {
		t.Fatalf("Claude Code = %#v, want %s in %s", got, program, extensions)
	}
	if got := byID[codex.ID]; got.Installed || got.Program != "" || got.Extensions != empty {
		t.Fatalf("Codex = %#v, want %s without a program", got, empty)
	}
	out.Reset()
	if err := executeAdapter(t, runtime, "list"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"(the newest version in ", "No program in "} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("list lacks %q:\n%s", want, out)
		}
	}
}
