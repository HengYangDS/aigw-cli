package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// programNotFound is the stop of an enabled client that declares no program
// and of which discovery finds none: its problem, and the step that installs
// the client or declares its program.
func programNotFound(client string) *adapter.StopError {
	return adapter.ProgramNotFound(domain.NewConfig(clients.Admission()), client)
}

// statusOfCodex is Codex's entry in `aigw status --json`, with the status's
// own state and step.
type statusOfCodex struct {
	State, Detail, NextAction, NextDetail string
	FilesDeferred, FilesReady             bool
	Overall                               deferredActivationDocument
}

// codexStatus runs `aigw status --json` and returns Codex's entry.
func codexStatus(t *testing.T, app *cli.App, out *bytes.Buffer) statusOfCodex {
	t.Helper()
	out.Reset()
	if err := cli.Execute(app, []string{"status", "--json"}); err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	var document struct {
		Clients map[string]struct {
			State         string `json:"state"`
			Detail        string `json:"detail"`
			NextAction    string `json:"next_action"`
			NextDetail    string `json:"next_detail"`
			FilesDeferred bool   `json:"client_files_deferred"`
			FilesReady    bool   `json:"client_files_ready"`
		} `json:"clients"`
	}
	var overall deferredActivationDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil || json.Unmarshal(out.Bytes(), &overall) != nil {
		t.Fatalf("decode status: %v\n%s", err, out)
	}
	entry := document.Clients[codex.ID]
	return statusOfCodex{entry.State, entry.Detail, entry.NextAction, entry.NextDetail, entry.FilesDeferred, entry.FilesReady, overall}
}

// codexCheck runs `aigw check --depth local --json`, which fails exactly when
// failed is true, and returns Codex's climb.
func codexCheck(t *testing.T, app *cli.App, out *bytes.Buffer, failed bool) checkedClient {
	t.Helper()
	out.Reset()
	if err := cli.Execute(app, []string{"check", "--depth", "local", "--json"}); (err != nil) != failed {
		t.Fatalf("check --depth local = %v, want a failure: %v\n%s", err, failed, out)
	}
	checked, found := decodeCheck(t, out).client(codex.ID)
	if !found {
		t.Fatalf("check names no Codex:\n%s", out)
	}
	return checked
}

// filesCheck returns the climb's check of the client's files.
func filesCheck(t *testing.T, checked checkedClient) checkedLocal {
	t.Helper()
	index := slices.IndexFunc(checked.Checks, func(check checkedLocal) bool { return check.Name == codex.ID+":files" })
	if index < 0 {
		t.Fatalf("Codex's climb has no check of its files: %+v", checked)
	}
	return checked.Checks[index]
}

// A Codex that declares an extensions directory and no program path, whose
// files a sync wrote, is judged against AIGW's record: status and the local
// check report its files as projected.
func TestADeclaredExtensionsDirectoryIsJudgedAgainstTheRecord(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := extensionCodex(t)
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if status := codexStatus(t, app, out); status.State != "configured" || !status.FilesReady || status.FilesDeferred {
		t.Fatalf("status of Codex = %+v, want configured with its files ready", status)
	}
	checked := codexCheck(t, app, out, false)
	if checked.Failure != nil || checked.Depth != "local" || filesCheck(t, checked).State != "passed" {
		t.Fatalf("check of Codex = %+v, want its local depth passed", checked)
	}
}

// A Codex whose declared program is removed after a sync is reported as
// stopped by its missing program, with the stop's cause and repair, by status
// and the local check alike, never as a client whose files AIGW has not
// written.
func TestAStoppedClientIsNotReportedAsUnprojected(t *testing.T) {
	t.Parallel()
	app, out, _, pkg, _ := syncedCodex(t)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatal(err)
	}
	spec, _ := clients.Admission().Spec(codex.ID)
	repair := adapter.ProgramRepair(spec)
	status := codexStatus(t, app, out)
	if status.State != "invalid" || status.Detail != "Codex program is missing" || !wantNext(status.NextAction, status.NextDetail, repair) || status.FilesDeferred {
		t.Fatalf("status of Codex = %+v, want stopped by its missing program", status)
	}
	checked := codexCheck(t, app, out, true)
	files := filesCheck(t, checked)
	if checked.Failure == nil || checked.Failure.Problem != "Codex program is missing" || files.State != "failed" || files.Detail != checked.Failure.Problem ||
		!wantNext(checked.NextAction, checked.NextDetail, repair) || strings.Contains(out.String(), "has not written") {
		t.Fatalf("check of Codex = %+v, want stopped by its missing program", checked)
	}
}

// The operator deleted Codex's declaration instead of renaming it, after a
// sync wrote its files and AIGW's record. Status and the local check judge
// the files against that record, which they match, and name the step that
// gives Codex a program: a sync, which adopts the one that discovery finds,
// and when discovery finds none, the step that installs Codex or declares its
// program. The status is not deferred, since the files are written. The sync
// adopts the program, after which Codex is configured.
func TestAClientWhoseDeclarationWasDeletedKeepsItsRecord(t *testing.T) {
	t.Parallel()
	app, out, program, _, _ := syncedCodex(t)
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DeclareProgram(codex.ID, "", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	missing := programNotFound(codex.ID)
	for _, test := range []struct {
		name          string
		found         map[string]string
		problem, step string
	}{
		{"discovery finds none", nil, missing.Error() + "; its files match AIGW's record", missing.NextAction()},
		{"discovery finds Codex", map[string]string{codex.ID: program}, "Codex declares no program, and a sync adopts the one that discovery finds; its files match AIGW's record", "aigw sync"},
	} {
		name := test.name
		app.Discovery = fakeDiscovery{result: discovery.Result{Executables: test.found}}
		status := codexStatus(t, app, out)
		if status.State != "deferred" || status.Detail != test.problem || !wantNext(status.NextAction, status.NextDetail, test.step) ||
			!status.FilesReady || status.FilesDeferred || status.Overall.State == "deferred" || !wantNext(status.Overall.NextAction, status.Overall.NextDetail, test.step) {
			t.Fatalf("%s: status of Codex = %+v", name, status)
		}
		checked := codexCheck(t, app, out, true)
		if files := filesCheck(t, checked); checked.Failure == nil || checked.Failure.Problem != test.problem || checked.State != "deferred" ||
			files.Detail != test.problem || !wantNext(checked.NextAction, checked.NextDetail, test.step) ||
			!slices.ContainsFunc(checked.Checks, func(check checkedLocal) bool { return check.Name == "codex:target-1" && check.State == "passed" }) {
			t.Fatalf("%s: check of Codex = %+v", name, checked)
		}
	}
	out.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if adopted, err := app.Config.Load(); err != nil || adopted.Clients[codex.ID].Program != program {
		t.Fatalf("the sync adopted %q, %v; want %q", adopted.Clients[codex.ID].Program, err, program)
	}
	if status := codexStatus(t, app, out); status.State != "configured" {
		t.Fatalf("status after the adoption = %+v, want configured", status)
	}
}

// On a host whose only Codex is an editor's extension, which no discovery
// adopts even when its folder is on the PATH, status, the local check and a
// sync name the step that installs Codex or declares its program, which names
// --extensions, rather than a sync alone, which would find nothing; the sync
// adopts nothing. The declaration of the extensions directory then
// synchronizes Codex.
func TestAClientWithoutAUsableProgramNamesItsDeclaration(t *testing.T) {
	t.Parallel()
	app, out, extensions, program, _ := extensionCodex(t)
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DeclareProgram(codex.ID, "", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Discovery = adapter.NewDiscoverer(clients.Registry(), discovery.System{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Home: app.Host.Home, Path: filepath.Dir(program)})
	missing := programNotFound(codex.ID)
	if !strings.Contains(missing.NextAction(), "--extensions <dir>") || !strings.Contains(missing.NextAction(), "--program <path>") {
		t.Fatalf("the missing program's step %q does not name both declarations", missing.NextAction())
	}
	status := codexStatus(t, app, out)
	if status.State != "deferred" || status.Detail != missing.Error() || !wantNext(status.NextAction, status.NextDetail, missing.NextAction()) || !status.FilesDeferred {
		t.Fatalf("status of Codex = %+v", status)
	}
	checked := codexCheck(t, app, out, true)
	if files := filesCheck(t, checked); checked.Failure == nil || files.Detail != missing.Error() || !wantNext(checked.NextAction, checked.NextDetail, missing.NextAction()) {
		t.Fatalf("check of Codex = %+v", checked)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"sync", "--json"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	var synced deferredActivationDocument
	if err := json.Unmarshal(out.Bytes(), &synced); err != nil || !wantNext(synced.NextAction, synced.NextDetail, missing.NextAction()) {
		t.Fatalf("sync's step = %+v, %v\n%s", synced, err, out)
	}
	if kept, err := app.Config.Load(); err != nil || kept.Clients[codex.ID].DeclaresProgram() {
		t.Fatalf("the sync adopted %+v, %v; want no program", kept.Clients[codex.ID], err)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"client", "edit", codex.ID, "--extensions", extensions}); err != nil {
		t.Fatalf("client edit --extensions: %v\n%s", err, out)
	}
	if status := codexStatus(t, app, out); status.State != "configured" || !status.FilesReady {
		t.Fatalf("status after the declaration = %+v, want configured", status)
	}
}
