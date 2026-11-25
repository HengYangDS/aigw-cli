package cli_test

import (
	"bytes"
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
	"aigw-cli/internal/terminal"
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

// stoppedFor requires err to be Codex's stop for its missing executable, with
// the fix that names another one.
func stoppedFor(t *testing.T, command string, err error) terminal.Problem {
	t.Helper()
	stop, stopped := errors.AsType[*adapter.StopError](err)
	problem := terminal.ProblemFor(err)
	if !stopped || stop.Client() != codex.ID || problem.Title != "Codex executable is missing" || problem.Fix != "aigw client edit codex --executable <path>" {
		t.Fatalf("%s = %v (%+v), want Codex stopped for its missing executable", command, err, problem)
	}
	return problem
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
		stoppedFor(t, "sync", cli.Execute(app, []string{"sync"}))
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
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after the edit: %v", err)
	}
	if record := read(t, files[1:2])[0]; !strings.Contains(string(record), `"catalog_client_path": "`+next+`"`) {
		t.Fatalf("the record does not name the new executable:\n%s", record)
	}
}

// While Codex is stopped, an edit that drops its configuration file still
// withdraws AIGW's settings, record and model table from that file, which
// needs no program; the new file is not written, and the edit fails with the
// stop.
func TestADroppedFileIsWithdrawnWhileCodexIsStopped(t *testing.T) {
	t.Parallel()
	app, _, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(moved, []byte("model = \"own\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := codexFiles(t, target)
	stoppedFor(t, "client edit", cli.Execute(app, []string{"client", "edit", "codex", "--target", moved}))
	if contents := read(t, old); string(contents[0]) != "model_provider = \"native\"\n" || contents[1] != nil || contents[2] != nil {
		t.Fatalf("the dropped file keeps AIGW's settings: %q, record %t, table %t", contents[0], contents[1] != nil, contents[2] != nil)
	}
	if contents := read(t, codexFiles(t, moved)); string(contents[0]) != "model = \"own\"\n" || contents[1] != nil || contents[2] != nil {
		t.Fatalf("the new file was written while Codex is stopped: %q", contents[0])
	}
}

// A selection and a rollback that would write a stopped Codex's files save
// the configuration, keep Codex's files and fail with the stop, saying what
// they wrote.
func TestUseAndRollbackReportAStoppedCodex(t *testing.T) {
	t.Parallel()
	app, _, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	files := codexFiles(t, target)
	before := read(t, files)
	for _, step := range []struct {
		command []string
		model   string
		impact  string
	}{
		{command: []string{"use", "codex", "gateway/gpt-other"}, model: "gpt-other", impact: "The selection was saved"},
		{command: []string{"config", "rollback", "--last-change"}, model: "gpt-test", impact: "aigw.toml was restored"},
	} {
		problem := stoppedFor(t, strings.Join(step.command[:2], " "), cli.Execute(app, step.command))
		if !strings.HasPrefix(problem.Impact, step.impact) || !strings.Contains(problem.Impact, "The files of Codex were not written") {
			t.Fatalf("%v impact = %q", step.command, problem.Impact)
		}
		cfg, err := app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		if selected := cfg.SelectedModel(codex.ID); selected != domain.NewModelRef("gateway", step.model) {
			t.Fatalf("%v left the selection %s, want gateway/%s", step.command, selected, step.model)
		}
		if after := read(t, files); !slices.EqualFunc(after, before, bytes.Equal) {
			t.Fatalf("%v changed Codex's files", step.command)
		}
	}
}

// A provider edit that moves Codex's endpoint while Codex is stopped saves the
// edit, keeps Codex's files and fails with the stop. A provider added then has
// no model, so it changes no client's files and stops none: it succeeds, and
// Codex's files stay as they were.
func TestProviderCommandsReportAStoppedCodex(t *testing.T) {
	t.Parallel()
	app, _, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	files := codexFiles(t, target)
	before := read(t, files)
	app.In = strings.NewReader("other-token\n")
	if err := cli.Execute(app, []string{"provider", "add", "other", "--endpoint", "anthropic=https://other.test", "--token-stdin"}); err != nil {
		t.Fatalf("provider add = %v, want a success that stops no client", err)
	}
	problem := stoppedFor(t, "provider edit", cli.Execute(app, []string{"provider", "edit", "gateway", "--endpoint", "responses=https://moved.gateway.test/v1"}))
	if !strings.HasPrefix(problem.Impact, "The edit of provider gateway was saved.") {
		t.Fatalf("provider edit impact = %q", problem.Impact)
	}
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
	app, _, _, pkg, target := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	team := filepath.Join(t.TempDir(), "team.toml")
	writeFile(t, team, []byte(`version = 8

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
	problem := stoppedFor(t, "setup", cli.Execute(app, []string{"setup", "--from", team}))
	if !strings.HasPrefix(problem.Impact, "Setup saved the configuration") {
		t.Fatalf("setup impact = %q", problem.Impact)
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
