package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// aliasedCodex configures Codex with targets, where alias.toml links to
// work/codex.toml, and returns the app, its output and the two paths.
func aliasedCodex(t *testing.T, named func(file, alias string) []string) (*cli.App, interface {
	Reset()
	String() string
}, string, string,
) {
	t.Helper()
	app, output, store, _, _ := testApp(t, "")
	dir := t.TempDir()
	file := filepath.Join(dir, "work", "codex.toml")
	writeFile(t, file, []byte("model = \"gpt-native\"\n"), 0o600)
	alias := filepath.Join(dir, "alias.toml")
	if err := os.Symlink(file, alias); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(codex.HomeDefault), ConfigPath: filepath.Join(dir, ".codex", "config.toml"), Present: true, AutoManaged: true}}}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Models["sol"] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedModel(codex.ID, "sol", "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), named(file, alias))
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	return app, output, file, alias
}

// A target through a symbolic link is supported. Its record lives beside the
// resolved file, so sync and check read the record that sync wrote, and the
// alias named beside the file it links to converges into one target, which
// ended the loop of a fix that sync refused again.
func TestASymlinkedCodexTargetSynchronizesAndChecks(t *testing.T) {
	for name, named := range map[string]func(file, alias string) []string{
		"the alias alone":           func(_, alias string) []string { return []string{alias} },
		"the alias beside its file": func(file, alias string) []string { return []string{file, alias} },
		"the file beside its alias": func(file, alias string) []string { return []string{alias, file} },
	} {
		t.Run(name, func(t *testing.T) {
			app, output, file, alias := aliasedCodex(t, named)
			syncAndCheckPass(t, app, output, codex.ID)
			resolved, err := filepath.EvalSymlinks(file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(resolved + ".aigw-state.json"); err != nil {
				t.Fatalf("no record beside the resolved file: %v", err)
			}
			if _, err := os.Lstat(alias + ".aigw-state.json"); !os.IsNotExist(err) {
				t.Fatalf("a second record beside the link: %v", err)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			kept := 0
			for _, target := range cfg.Clients[codex.ID].Targets {
				if target == file || target == alias {
					kept++
				}
			}
			if kept != 1 {
				t.Fatalf("the file and its alias are kept %d times: %q", kept, cfg.Clients[codex.ID].Targets)
			}
		})
	}
}
