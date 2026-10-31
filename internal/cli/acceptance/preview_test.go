package cli_test

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/program"
	"aigw-cli/internal/secret"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Preview and PreviewCandidate report the fake's preview result; neither
// installs anything.
func (u *fakeUpdater) Preview(_ context.Context, _ string) (program.Transition, error) {
	u.previewCalls++
	return u.previewResult, u.previewErr
}

func (u *fakeUpdater) PreviewCandidate(_ string, candidate program.CandidateArchive) (program.Transition, error) {
	u.previewCalls++
	u.candidateReceived = candidate
	return u.previewResult, u.previewErr
}

// errCredentialTouched fails any use of a credential store that a preview
// must not read, check or write.
var errCredentialTouched = errors.New("a preview used the credential store")

// TestSetupAndAddPreviewsUseNoCredential runs each Token-free preview with a
// credential store that fails every call: setup from flags and from a
// manifest that recommends the Route to Claude Code, and add. Each succeeds,
// calls the store not once, reports the credential reader that applying it
// would install, even where no client file changes, and writes neither the
// reader nor configuration.
func TestSetupAndAddPreviewsUseNoCredential(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "team.toml")
	writeFile(t, manifest, []byte(`version = 7
[recommendations.claude.primary]
route = "team-route"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.test"
[models.team-model]
label = "Team Model"
[routes.team-route]
label = "Team Route"
account = "team"
model = "team-model"
upstream_model = "team-model"
interfaces = { anthropic = ["text"] }
`), 0o600)
	for _, test := range []struct {
		name       string
		args       []string
		discovered bool
	}{
		{"setup", []string{"setup", "--account", "team", "--route", "team-route", "--anthropic-url", "https://team.test", "--model", "team-model", "--for", "claude", "--dry-run", "--json"}, false},
		{"setup from a manifest", []string{"setup", "--from", manifest, "--account", "team", "--dry-run", "--json"}, true},
		{"add", []string{"add", "team", "--anthropic-url", "https://team.test", "--model", "team-model", "--for", "claude", "--dry-run", "--json"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			if test.discovered {
				app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
			}
			writeFile(t, app.Executable, []byte("program"), 0o700)
			app.DataDir = filepath.Join(t.TempDir(), "data")
			reader, err := secret.VersionedEntrypointPath(app.DataDir, app.Executable, executableName("aigw"))
			if err != nil {
				t.Fatal(err)
			}
			app.CredentialPath = reader
			store := &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), getErr: errCredentialTouched, setErr: errCredentialTouched, existsErr: errCredentialTouched}
			app.Secrets = store
			if err := cli.Execute(app, test.args); err != nil {
				t.Fatalf("preview failed: %v (get %q, exists %q, set %q)\n%s", err, store.getCalls, store.existsCalls, store.setCalls, out.String())
			}
			if calls := len(store.getCalls) + len(store.existsCalls) + len(store.setCalls) + len(store.deleteCalls); calls != 0 {
				t.Fatalf("preview used the credential store %d times: get %q, exists %q, set %q", calls, store.getCalls, store.existsCalls, store.setCalls)
			}
			var document struct {
				DryRun               bool `json:"dry_run"`
				CredentialEntrypoint struct {
					Path string `json:"path"`
				} `json:"credential_entrypoint"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil || !document.DryRun || document.CredentialEntrypoint.Path != reader {
				t.Fatalf("preview document = %s, %v; want the reader %s", out.String(), err, reader)
			}
			for _, path := range []string{app.Config.Path(), reader} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("preview wrote %s: %v", path, err)
				}
			}
		})
	}
}

// TestAddPreviewRefusesAReadOnlyBackend shows that add --dry-run refuses
// where add would: a read-only backend cannot store the new Account's Token.
func TestAddPreviewRefusesAReadOnlyBackend(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	err := cli.Execute(app, []string{"add", "team", "--anthropic-url", "https://team.test", "--model", "team-model", "--for", "claude", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("read-only add preview = %v", err)
	}
	if _, err := os.Stat(app.Config.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused preview wrote configuration: %v", err)
	}
}

// TestUpdatePreviewInstallsNothing shows that update --dry-run asks only for
// the preview, reports the installed and the newer version with the program
// file and rollback copy that the update would write, and refuses to preview
// a rollback.
func TestUpdatePreviewInstallsNothing(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	writeFile(t, app.Executable, []byte("program"), 0o700)
	updater := &fakeUpdater{previewResult: program.Transition{State: program.TransitionAvailable, Version: "0.2.0", Summary: "would update to v0.2.0"}}
	app.Updater = updater
	if err := cli.Execute(app, []string{"update", "--dry-run", "--json"}); err != nil {
		t.Fatalf("update preview failed: %v\n%s", err, out.String())
	}
	if updater.previewCalls != 1 || updater.updateCalls+updater.candidateCalls+updater.rollbackCalls != 0 {
		t.Fatalf("calls: preview %d, update %d, candidate %d, rollback %d", updater.previewCalls, updater.updateCalls, updater.candidateCalls, updater.rollbackCalls)
	}
	var document struct {
		DryRun         bool   `json:"dry_run"`
		State          string `json:"state"`
		Version        string `json:"version"`
		CurrentVersion string `json:"current_version"`
		Path           string `json:"path"`
		Rollback       string `json:"rollback"`
		NextAction     string `json:"next_action"`
		NextDetail     string `json:"next_detail"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	want := program.RollbackPath(app.Executable)
	if !document.DryRun || document.State != program.TransitionAvailable || document.Version != "0.2.0" || document.CurrentVersion != app.Version ||
		document.Path != app.Executable || document.Rollback != want || document.NextAction != "" || !strings.Contains(document.NextDetail, "with this same program") {
		t.Fatalf("update preview document = %s", out.String())
	}
	if _, err := os.Stat(want); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("update preview wrote the rollback copy: %v", err)
	}
	out.Reset()
	err := cli.Execute(app, []string{"update", "--dry-run", "--rollback"})
	if err == nil || !strings.Contains(err.Error(), "dry-run") || updater.rollbackCalls != 0 {
		t.Fatalf("rollback preview = %v, rollback calls %d", err, updater.rollbackCalls)
	}
}

// TestProgramPreviewsWriteNothing shows that install --dry-run creates neither
// the target nor its directory, and uninstall --dry-run keeps the program file
// and its rollback copy that it names.
func TestProgramPreviewsWriteNothing(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	writeFile(t, app.Executable, []byte("program"), 0o700)
	if err := cli.Execute(app, []string{"install", "--dry-run", "--json"}); err != nil {
		t.Fatalf("install preview failed: %v\n%s", err, out.String())
	}
	if _, err := os.Stat(filepath.Dir(app.InstallTarget)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("install preview created the target directory: %v", err)
	}
	installed := filepath.Join(t.TempDir(), "aigw")
	writeFile(t, installed, []byte("installed"), 0o700)
	writeFile(t, program.RollbackPath(installed), []byte("previous"), 0o700)
	out.Reset()
	if err := cli.Execute(app, []string{"uninstall", "--target", installed, "--dry-run", "--json"}); err != nil {
		t.Fatalf("uninstall preview failed: %v\n%s", err, out.String())
	}
	var document struct {
		Removed []string `json:"removed"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil || len(document.Removed) != 2 {
		t.Fatalf("uninstall preview document = %s, %v", out.String(), err)
	}
	for _, path := range []string{installed, program.RollbackPath(installed)} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("uninstall preview removed %s: %v", path, err)
		}
	}
}

// TestUninstallPreviewNamesTheWithdrawal shows that uninstall --dry-run with
// configured clients names the clients that uninstalling would withdraw and
// the client file that withdrawal would change, and changes neither that file,
// the configuration nor the program.
func TestUninstallPreviewNamesTheWithdrawal(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	writeFile(t, app.Executable, []byte("program"), 0o700)
	codexTarget := filepath.Join(t.TempDir(), "config.toml")
	configureUninstallClients(t, app, codexTarget)
	files := map[string][]byte{}
	for _, path := range []string{app.Config.Path(), codexTarget, app.Executable} {
		files[path] = readFile(t, path)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"uninstall", "--dry-run", "--json"}); err != nil {
		t.Fatalf("uninstall preview failed: %v\n%s", err, out.String())
	}
	var document struct {
		DryRun           bool                     `json:"dry_run"`
		WithdrawnClients []string                 `json:"withdrawn_clients"`
		Removed          []string                 `json:"removed"`
		Targets          []adapter.ProjectionPlan `json:"targets"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("uninstall preview document = %s, %v", out.String(), err)
	}
	codexFile, err := os.Stat(codexTarget)
	if err != nil {
		t.Fatal(err)
	}
	// The target may be reported through the temporary directory's resolved
	// path, so the comparison is by file.
	withdrawsCodex := slices.ContainsFunc(document.Targets, func(plan adapter.ProjectionPlan) bool {
		target, err := os.Stat(plan.Target)
		return plan.Client == codex.ID && err == nil && os.SameFile(target, codexFile)
	})
	if !document.DryRun || !slices.Equal(document.WithdrawnClients, []string{claude.ID, codex.ID}) || !slices.Equal(document.Removed, []string{app.Executable}) || !withdrawsCodex {
		t.Fatalf("uninstall preview document = %s", out.String())
	}
	for path, want := range files {
		if got := readFile(t, path); !bytes.Equal(got, want) {
			t.Fatalf("uninstall preview changed %s", path)
		}
	}
}
