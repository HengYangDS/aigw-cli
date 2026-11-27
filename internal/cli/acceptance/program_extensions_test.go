package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/processtest"
)

// installCodexExtension installs version of Codex's extension into the
// editor's extensions directory extensions, with fakeCodex as its program at
// the path that the extension declares for this host, and returns the
// program.
func installCodexExtension(t *testing.T, extensions, version string) string {
	t.Helper()
	spec, _ := clients.Admission().Spec(codex.ID)
	inside := strings.NewReplacer("{os}", spec.Extension.OS[runtime.GOOS], "{arch}", spec.Extension.Arch[runtime.GOARCH]).Replace(spec.Extension.Program)
	program := filepath.Join(extensions, spec.Extension.ID+"-"+version, filepath.FromSlash(inside))
	processtest.Write(t, program, fakeCodex)
	return program
}

// extensionCodex is an App whose Codex is enabled for a configuration file
// outside Codex's default home, selects gateway/gpt-test, whose provider has
// its Token, and declares an editor's extensions directory that holds version
// 26.5.1 of Codex's extension, which no discovery finds. Nothing is
// synchronized yet. It returns the App, its output, the directory, the
// version's program and the file.
func extensionCodex(t *testing.T) (*cli.App, *bytes.Buffer, string, string, string) {
	t.Helper()
	app, out, secrets, _, _ := testApp(t, "")
	app.Host.GOARCH = runtime.GOARCH
	extensions := t.TempDir()
	program := installCodexExtension(t, extensions, "26.5.1")
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model_provider = \"native\"\n"), 0o600)
	cfg := domain.NewConfig(clients.Admission())
	selected := addProviderModel(&cfg, "gateway", "Gateway", domain.Endpoints{Responses: "https://gateway.test/v1"}, codex.ID, "gpt-test")
	cfg.SetSelectedModel(codex.ID, selected, "")
	cfg.SetClientActivation(codex.ID, true, "", []string{target})
	cfg.DeclareProgram(codex.ID, "", extensions)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	return app, out, extensions, program, target
}

// codexProducer reads, from AIGW's record of Codex's configuration file
// target, the program that produced Codex's model table, and whether the
// record holds a table.
func codexProducer(t *testing.T, target string) (string, bool) {
	t.Helper()
	var record struct {
		Table    string `json:"catalog_hash"`
		Producer string `json:"catalog_client_path"`
	}
	if err := json.Unmarshal(readFile(t, codexFiles(t, target)[1]), &record); err != nil {
		t.Fatalf("decode AIGW's record of %s: %v", target, err)
	}
	return record.Producer, record.Table != ""
}

// listedClient is one item of `aigw client list --json`.
type listedClient struct {
	ID         string `json:"id"`
	Program    string `json:"program"`
	Extensions string `json:"extensions"`
	Installed  bool   `json:"installed"`
}

// listedCodex is Codex's item in `aigw client list --json`.
func listedCodex(t *testing.T, app *cli.App, out *bytes.Buffer) listedClient {
	t.Helper()
	out.Reset()
	if err := cli.Execute(app, []string{"client", "list", "--json"}); err != nil {
		t.Fatalf("client list: %v\n%s", err, out)
	}
	var list struct {
		Clients []listedClient `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatalf("decode client list: %v\n%s", err, out)
	}
	index := slices.IndexFunc(list.Clients, func(item listedClient) bool { return item.ID == codex.ID })
	if index < 0 {
		t.Fatalf("client list has no Codex:\n%s", out)
	}
	return list.Clients[index]
}

// Codex, the client whose program runs during a sync and whose record names
// that program, declares an editor's extensions directory: the sync runs the
// program of the newest installed version for Codex's model table, writes the
// table with Codex's file, and AIGW's record names that program as the
// table's producer, while aigw.toml keeps its extensions and gains no program.
// When the editor installs a newer version, the next sync uses the new
// version's program while aigw.toml keeps its bytes, and the client list names
// the new program beside the declared directory.
func TestCodexExtensionsDirectoryProducesTheModelTable(t *testing.T) {
	t.Parallel()
	app, out, extensions, program, target := extensionCodex(t)
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if producer, table := codexProducer(t, target); producer != program || !table {
		t.Fatalf("the record names %q as the producer of its table (held: %v), want %q", producer, table, program)
	}
	if catalog := read(t, codexFiles(t, target)[2:])[0]; !bytes.Contains(catalog, []byte("gpt-test")) {
		t.Fatalf("Codex's model table is not the program's:\n%s", catalog)
	}
	declared := readFile(t, app.Config.Path())
	saved, err := app.Config.Load()
	if binding := saved.Clients[codex.ID]; err != nil || binding.Program != "" || binding.Extensions != extensions {
		t.Fatalf("saved Codex = %+v, %v; want its extensions and no program", binding, err)
	}

	newer := installCodexExtension(t, extensions, "26.10.0")
	out.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after the newer version: %v\n%s", err, out)
	}
	if producer, table := codexProducer(t, target); producer != newer || !table {
		t.Fatalf("the record names %q as the producer of its table (held: %v), want the newer %q", producer, table, newer)
	}
	if after := readFile(t, app.Config.Path()); !bytes.Equal(after, declared) {
		t.Fatalf("the sync changed aigw.toml:\n%s", after)
	}
	if listed := listedCodex(t, app, out); listed.Program != newer || listed.Extensions != extensions || !listed.Installed {
		t.Fatalf("client list names %+v, want %q in %q", listed, newer, extensions)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"client", "list"}); err != nil {
		t.Fatalf("client list: %v\n%s", err, out)
	}
	if !strings.Contains(out.String(), newer+" (the newest version in "+extensions+")") {
		t.Fatalf("client list does not name the new program beside the directory:\n%s", out)
	}
}
