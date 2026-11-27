package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/processtest"
)

// fakeCodex stands for Codex: it prints its version and a model table that
// holds the selected models.
const fakeCodex = `#!/bin/sh
case "$1" in
--version) echo "codex-cli 1.0.0" ;;
debug) echo '{"models":[{"slug":"gpt-test","display_name":"GPT"},{"slug":"gpt-other","display_name":"Other"}]}' ;;
*) exit 2 ;;
esac
`

// syncedCodex is an App whose Codex `aigw sync` synchronized, with a model
// table, through a fake Codex in a package folder, at a configuration file
// outside Codex's default home. It returns the App, its output, the program,
// its package folder and the file.
func syncedCodex(t *testing.T) (*cli.App, *bytes.Buffer, string, string, string) {
	t.Helper()
	app, out, secrets, _, _ := testApp(t, "")
	pkg := filepath.Join(t.TempDir(), "codex-1.0.0")
	program := filepath.Join(pkg, "bin", "codex")
	processtest.Write(t, program, fakeCodex)
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	endpoints := domain.Endpoints{Responses: "https://gateway.test/v1"}
	selected := addProviderModel(&cfg, "gateway", "Gateway", endpoints, codex.ID, "gpt-test")
	addProviderModel(&cfg, "gateway", "Gateway", endpoints, codex.ID, "gpt-other")
	cfg.SetSelectedModel(codex.ID, selected, "")
	cfg.SetClientActivation(codex.ID, true, program, []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("the first sync: %v\n%s", err, out)
	}
	for _, path := range codexFiles(t, target) {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("the first sync wrote no %s: %v", path, err)
		}
	}
	out.Reset()
	return app, out, program, pkg, target
}

// codexFiles are a Codex configuration file, AIGW's record of it and its
// model table.
func codexFiles(t *testing.T, target string) []string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	return []string{target, canonical + ".aigw-state.json", canonical + ".aigw-model-catalog.json"}
}

// read returns the bytes of each file, nil for one that is not there.
func read(t *testing.T, paths []string) [][]byte {
	t.Helper()
	contents := make([][]byte, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		contents = append(contents, data)
	}
	return contents
}

// stoppedFor requires err to fail the invocation for Codex's missing
// executable, and out, which it reads and empties, to end the command's own
// report with the stop: the problem and the fix that names another
// executable. It returns what the invocation wrote.
func stoppedFor(t *testing.T, out *bytes.Buffer, command string, err error) string {
	t.Helper()
	text := out.String()
	out.Reset()
	stop, stopped := errors.AsType[*adapter.StopError](err)
	if !stopped || stop.Client() != codex.ID || stop.Error() != "Codex executable is missing" || !strings.Contains(text, "Stopped clients") ||
		!strings.Contains(text, "Codex executable is missing") || !strings.Contains(text, "aigw client edit codex --executable <path>") {
		t.Fatalf("%s = %v, want Codex stopped for its missing executable after the command's report:\n%s", command, err, text)
	}
	return text
}

// requireLine requires text to hold want as a whole line, indentation aside.
func requireLine(t *testing.T, text, want string) {
	t.Helper()
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == want {
			return
		}
	}
	t.Fatalf("no line reads %q in:\n%s", want, text)
}

// After a sync, Codex's executable goes. Two syncs in a row each fail with
// the missing executable and its fix, and keep Codex's file, AIGW's record
// and the model table byte for byte; a check names the same problem and fix.
// Naming a runnable executable then ends the stop, and a sync records it as
// the table's producer.
func TestASyncedCodexWhoseExecutableGoesStopsUntilAnotherIsNamed(t *testing.T) {
	t.Parallel()
	app, out, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	files := codexFiles(t, target)
	before := read(t, files)
	for attempt := range 2 {
		stoppedFor(t, out, "sync", cli.Execute(app, []string{"sync"}))
		if after := read(t, files); !slices.EqualFunc(after, before, bytes.Equal) {
			t.Fatalf("sync %d changed Codex's files", attempt+2)
		}
	}
	out.Reset()
	if err := cli.Execute(app, []string{"check", "codex", "--depth", "local"}); err == nil ||
		!strings.Contains(out.String(), "Codex executable is missing") || !strings.Contains(out.String(), "aigw client edit codex --executable <path>") {
		t.Fatalf("check = %v, want the missing executable and its fix:\n%s", err, out)
	}

	next := filepath.Join(t.TempDir(), "codex-1.1.0", "bin", "codex")
	processtest.Write(t, next, fakeCodex)
	if err := cli.Execute(app, []string{"client", "edit", "codex", "--executable", next}); err != nil {
		t.Fatalf("client edit with a runnable executable: %v", err)
	}
	// The edit itself synchronizes Codex with the new executable.
	if record := read(t, files[1:2])[0]; !strings.Contains(string(record), `"catalog_client_path": "`+next+`"`) {
		t.Fatalf("the record does not name the new executable after the edit:\n%s", record)
	}
}

// While Codex is stopped, an edit that drops its configuration file still
// withdraws AIGW's settings, record and model table from that file, which
// needs no program; the new file is not written, and the edit fails with the
// stop.
func TestADroppedFileIsWithdrawnWhileCodexIsStopped(t *testing.T) {
	t.Parallel()
	app, out, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(moved, []byte("model = \"own\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := codexFiles(t, target)
	text := stoppedFor(t, out, "client edit", cli.Execute(app, []string{"client", "edit", "codex", "--target", moved}))
	requireLine(t, text, "AIGW withdrew its settings from the files of Codex that the change drops and left the others as they were.")
	if contents := read(t, old); string(contents[0]) != "model_provider = \"native\"\n" || contents[1] != nil || contents[2] != nil {
		t.Fatalf("the dropped file keeps AIGW's settings: %q, record %t, table %t", contents[0], contents[1] != nil, contents[2] != nil)
	}
	if contents := read(t, codexFiles(t, moved)); string(contents[0]) != "model = \"own\"\n" || contents[1] != nil || contents[2] != nil {
		t.Fatalf("the new file was written while Codex is stopped: %q", contents[0])
	}
}

// A selection, its preview and a rollback that would write a stopped Codex's
// files keep their own reports, the selection's JSON fields among them, keep
// Codex's files, and are followed by the stop, which says only what the
// record holds, here that Codex's files were left as they were, and fails.
func TestUseAndRollbackReportAStoppedCodex(t *testing.T) {
	t.Parallel()
	app, out, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	files := codexFiles(t, target)
	before := read(t, files)
	selected := func(model string) {
		t.Helper()
		cfg, err := app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.SelectedModel(codex.ID); got != domain.NewModelRef("gateway", model) {
			t.Fatalf("the selection is %s, want gateway/%s", got, model)
		}
		if after := read(t, files); !slices.EqualFunc(after, before, bytes.Equal) {
			t.Fatal("a command changed Codex's files")
		}
	}

	err := cli.Execute(app, []string{"use", "codex", "gateway/gpt-other", "--json"})
	var document struct {
		OK      bool   `json:"ok"`
		Model   string `json:"model"`
		Stopped []struct {
			Client     string `json:"client"`
			Problem    string `json:"problem"`
			Files      string `json:"files"`
			NextDetail string `json:"next_detail"`
		} `json:"stopped_clients"`
	}
	if decodeErr := json.Unmarshal(out.Bytes(), &document); decodeErr != nil || !errors.As(err, new(*adapter.StopError)) {
		t.Fatalf("use --json = %v, %v:\n%s", err, decodeErr, out)
	}
	out.Reset()
	if document.OK || document.Model == "" || len(document.Stopped) != 1 || document.Stopped[0].Client != codex.ID ||
		document.Stopped[0].Files != "left as they were" || document.Stopped[0].NextDetail != "aigw client edit codex --executable <path>" {
		t.Fatalf("use --json = %+v, want its own fields and Codex stopped", document)
	}
	selected("gpt-other")

	text := stoppedFor(t, out, "use --dry-run", cli.Execute(app, []string{"use", "codex", "gateway/gpt-test", "--dry-run"}))
	requireLine(t, text, "A run would leave the files of Codex as they are.")
	selected("gpt-other")

	text = stoppedFor(t, out, "config rollback", cli.Execute(app, []string{"config", "rollback", "--last-change"}))
	requireLine(t, text, "✓ aigw.toml was restored; clients were not restarted.")
	requireLine(t, text, "AIGW left the files of Codex as they were.")
	selected("gpt-test")
}

// A provider edit that moves Codex's endpoint while Codex is stopped saves the
// edit, keeps Codex's files and fails with the stop. A provider added then has
// no model, so it changes no client's files and stops none: it succeeds, and
// Codex's files stay as they were.
func TestProviderCommandsReportAStoppedCodex(t *testing.T) {
	t.Parallel()
	app, out, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	files := codexFiles(t, target)
	before := read(t, files)
	app.In = strings.NewReader("other-token\n")
	if err := cli.Execute(app, []string{"provider", "add", "other", "--endpoint", "anthropic=https://other.test", "--token-stdin"}); err != nil {
		t.Fatalf("provider add = %v, want a success that stops no client", err)
	}
	out.Reset()
	text := stoppedFor(t, out, "provider edit", cli.Execute(app, []string{"provider", "edit", "gateway", "--endpoint", "responses=https://moved.gateway.test/v1"}))
	requireLine(t, text, "✓ Saved; the provider's Token is unchanged")
	requireLine(t, text, "AIGW left the files of Codex as they were.")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, added := cfg.Providers["other"]; !added || cfg.Providers["gateway"].Endpoints.Responses != "https://moved.gateway.test/v1" {
		t.Fatalf("the provider commands saved %+v", cfg.Providers)
	}
	if after := read(t, files); !slices.EqualFunc(after, before, bytes.Equal) {
		t.Fatal("a provider command changed Codex's files")
	}
}

// A team file applied while Codex is stopped saves what it changes, keeps
// Codex's files as they were though the file renames Codex's model, and fails
// with the stop.
func TestSetupReportsAStoppedCodex(t *testing.T) {
	t.Parallel()
	app, out, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	team := filepath.Join(t.TempDir(), "team.toml")
	writeFile(t, team, []byte(`version = 9

[[providers]]
id = "gateway"
label = "Gateway"
endpoints = {responses = "https://moved.gateway.test/v1"}

[[providers.models]]
id = "gpt-test"
family = "gpt-test"
label = "GPT Test"
protocols = {responses = []}

[[providers.models]]
id = "gpt-other"
family = "gpt-other"
label = "GPT Other"
protocols = {responses = []}

[[families]]
id = "gpt-test"
label = "GPT Test"

[[families]]
id = "gpt-other"
label = "GPT Other"

[recommendations.codex]
primary = {model = "gateway/gpt-test"}
`), 0o600)
	files := codexFiles(t, target)
	before := read(t, files)
	text := stoppedFor(t, out, "setup", cli.Execute(app, []string{"setup", "--from", team}))
	if !strings.Contains(text, "Team file applied") || strings.Index(text, "Team file applied") > strings.Index(text, "Stopped clients") {
		t.Fatalf("setup's own report does not come first:\n%s", text)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if label := cfg.Models[domain.NewModelRef("gateway", "gpt-test")].Label; label != "GPT Test" {
		t.Fatalf("setup saved the label %q, want the team file's", label)
	}
	if after := read(t, files); !slices.EqualFunc(after, before, bytes.Equal) {
		t.Fatal("setup changed Codex's files")
	}
}
