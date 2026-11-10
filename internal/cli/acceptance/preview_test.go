package cli_test

import (
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
	"fmt"
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

// TestSetupPreviewsUseNoCredential runs each Token-free preview with a
// credential store that reads and writes nothing: setup from flags and from a
// team file that recommends the model to Claude Code. Each succeeds,
// reports the credential reader that applying it would install, even where
// no client file changes, and writes neither the reader nor configuration.
// Only team setup's preview observes whether its provider has a Token, from
// the store's metadata, as the run does.
func TestSetupPreviewsUseNoCredential(t *testing.T) {
	manifest := recommendingManifest(t)
	for _, test := range []struct {
		name       string
		args       []string
		discovered bool
	}{
		{"setup from a team file", []string{"setup", "--from", manifest, "--dry-run", "--json"}, true},
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
			app.ResolveCredentialPath = func() (string, error) { return reader, nil }
			store := &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), getErr: errCredentialTouched, setErr: errCredentialTouched, existsErr: errCredentialTouched}
			if test.discovered {
				backend := secret.NewMemoryStore()
				if err := backend.Set("team", "fixture-token"); err != nil {
					t.Fatal(err)
				}
				store = &recordingCredentialStore[string]{backend: backend, getErr: errCredentialTouched, setErr: errCredentialTouched}
			}
			app.Secrets = store
			if err := cli.Execute(app, test.args); err != nil {
				t.Fatalf("preview failed: %v (get %q, exists %q, set %q)\n%s", err, store.getCalls, store.existsCalls, store.setCalls, out.String())
			}
			observed := len(store.existsCalls)
			if test.discovered {
				observed = 0
			}
			if calls := len(store.getCalls) + observed + len(store.setCalls) + len(store.deleteCalls); calls != 0 {
				t.Fatalf("preview used the credential store %d times: get %q, exists %q, set %q", calls, store.getCalls, store.existsCalls, store.setCalls)
			}
			// Team setup names the reader as the newer documents do.
			var document struct {
				DryRun               bool `json:"dry_run"`
				CredentialEntrypoint struct {
					Path string `json:"path"`
				} `json:"credential_entrypoint"`
				CredentialReader struct {
					Path string `json:"path"`
				} `json:"credential_reader"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil || !document.DryRun || document.CredentialEntrypoint.Path+document.CredentialReader.Path != reader {
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

// recommendingManifest writes a team manifest whose one Route Claude Code
// recommends.
func recommendingManifest(t *testing.T) string {
	t.Helper()
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
	return manifest
}

// TestSetupManifestPreviewInText shows the human preview of setup --from,
// which plans as if every provider's Token were stored, and its refusals of
// an unknown provider and of --token-stdin with a read-only backend.
func TestSetupManifestPreviewInText(t *testing.T) {
	manifest := recommendingManifest(t)
	app, out, _, _, _ := testApp(t, "")
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
	if err := cli.Execute(app, []string{"setup", "--from", manifest, "--dry-run"}); err != nil {
		t.Fatalf("manifest preview failed: %v\n%s", err, out.String())
	}
	for _, want := range []string{"Team file preview", "None; aigw provider rotate team", "Nothing was written"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("manifest preview lacks %q:\n%s", want, out.String())
		}
	}
	if err := cli.Execute(app, []string{"setup", "--from", manifest, "--provider", "missing", "--dry-run"}); err == nil || !strings.Contains(err.Error(), `unknown provider "missing"`) {
		t.Fatalf("unknown provider preview = %v", err)
	}
	// A named provider without a stored Token is planned as if the setup
	// stored the one that it would read: Claude Code takes the team's model,
	// and nothing is written or read.
	out.Reset()
	if err := cli.Execute(app, []string{"setup", "--from", manifest, "--provider", "team", "--dry-run", "--json"}); err != nil {
		t.Fatalf("a named provider preview: %v\n%s", err, out)
	}
	type plannedClient struct {
		ID    string `json:"id"`
		State string `json:"state"`
		Model string `json:"model"`
	}
	var named struct {
		DryRun  bool            `json:"dry_run"`
		Clients []plannedClient `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &named); err != nil || !named.DryRun {
		t.Fatalf("a named provider preview = %s, %v", out, err)
	}
	if index := slices.IndexFunc(named.Clients, func(client plannedClient) bool { return client.ID == claude.ID }); index < 0 ||
		named.Clients[index].State != "configured" || named.Clients[index].Model != "team/team-model" {
		t.Fatalf("a named provider preview plans Claude Code as %+v", named.Clients)
	}
	// A credential store that cannot tell whether the named provider has a
	// Token refuses the setup that would store one, before any input is read.
	unobservable := &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), existsErr: errors.New("locked")}
	app.Secrets = unobservable
	input := strings.NewReader("sk-never-read\n")
	app.In = input
	for _, args := range [][]string{
		{"setup", "--from", manifest, "--provider", "team", "--dry-run"},
		{"setup", "--from", manifest, "--provider", "team", "--token-stdin"},
	} {
		if err := cli.Execute(app, args); err == nil || !strings.Contains(err.Error(), "observe the Token of provider team: locked") {
			t.Fatalf("%s with a store that cannot tell = %v", strings.Join(args, " "), err)
		}
	}
	if len(unobservable.getCalls)+len(unobservable.setCalls) != 0 || input.Len() != len("sk-never-read\n") {
		t.Fatalf("a refused setup read %q, wrote %q and left %d input bytes", unobservable.getCalls, unobservable.setCalls, input.Len())
	}
	app.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"setup", "--from", manifest, "--provider", "team", "--token-stdin", "--dry-run"}, "the environment backend reads the team Token"},
	} {
		if err := cli.Execute(app, test.args); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s with a read-only backend = %v", strings.Join(test.args, " "), err)
		}
	}
	if _, err := os.Stat(app.Config.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a manifest preview wrote configuration: %v", err)
	}
}

// TestProviderAddPreviewRefusesAnEnvironmentWithoutTheToken shows that
// provider add --dry-run refuses where provider add would: the read-only
// environment backend has no Token for the new provider.
func TestProviderAddPreviewRefusesAnEnvironmentWithoutTheToken(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	err := cli.Execute(app, []string{"provider", "add", "team", "--endpoint", "anthropic=https://team.test", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "the environment backend has no team Token") {
		t.Fatalf("an environment preview without the Token = %v", err)
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
	if err := cli.Execute(app, []string{"self", "update", "--dry-run", "--json"}); err != nil {
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
	// Beside a program that retains no predecessor, a rollback preview has
	// nothing to name, and it runs no rollback.
	err := cli.Execute(app, []string{"self", "rollback", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "No previous program to roll back to") || updater.rollbackCalls != 0 {
		t.Fatalf("rollback preview = %v, rollback calls %d", err, updater.rollbackCalls)
	}
}

// TestUpdatePreviewInTextFromACandidateAndOnFailure shows the human update
// preview with its rollback copy, a candidate preview that receives the
// archive and installs nothing, and a failed preview that changed nothing.
func TestUpdatePreviewInTextFromACandidateAndOnFailure(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	writeFile(t, app.Executable, []byte("program"), 0o700)
	updater := &fakeUpdater{previewResult: program.Transition{State: program.TransitionAvailable, Version: "0.2.0", Summary: "would update to v0.2.0"}}
	app.Updater = updater
	if err := cli.Execute(app, []string{"self", "update", "--dry-run"}); err != nil {
		t.Fatalf("update preview failed: %v\n%s", err, out.String())
	}
	for _, want := range []string{"Update preview", "Rollback copy", "would update to v0.2.0", "with this same program"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("update preview lacks %q:\n%s", want, out.String())
		}
	}
	archive, checksums := filepath.Join(t.TempDir(), "aigw_0.2.0_darwin_arm64.tar.gz"), filepath.Join(t.TempDir(), "checksums.txt")
	if err := cli.Execute(app, []string{"self", "update", "--dry-run", "--archive", archive, "--checksums", checksums, "--json"}); err != nil {
		t.Fatalf("candidate preview failed: %v", err)
	}
	if updater.candidateReceived.ArchivePath != archive || updater.candidateReceived.ChecksumsPath != checksums || updater.candidateCalls+updater.updateCalls != 0 {
		t.Fatalf("candidate preview = %#v, installs %d", updater.candidateReceived, updater.candidateCalls+updater.updateCalls)
	}
	updater.previewErr = fmt.Errorf("metadata unavailable: %w", program.ErrReleaseSource)
	out.Reset()
	if err := cli.Execute(app, []string{"self", "update", "--dry-run"}); err == nil || !strings.Contains(out.String()+err.Error(), "Update preview did not complete") {
		t.Fatalf("failed preview = %v\n%s", err, out.String())
	}
	// A preview that cannot read release metadata can still preview a
	// verified local archive; doctor inspects neither.
	if !strings.Contains(out.String(), "Fix\n  aigw self update --dry-run --archive <archive> --checksums <checksums>") || strings.Contains(out.String(), "aigw check --depth local") {
		t.Fatalf("failed preview names another fix:\n%s", out.String())
	}
	// A preview of a named archive never names another archive.
	out.Reset()
	if err := cli.Execute(app, []string{"self", "update", "--dry-run", "--archive", archive, "--checksums", checksums}); err == nil ||
		!strings.Contains(out.String(), "Fix\n  aigw self status") || strings.Contains(out.String(), "<archive>") {
		t.Fatalf("failed candidate preview = %v\n%s", err, out.String())
	}
	if _, err := os.Stat(program.RollbackPath(app.Executable)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an update preview wrote the rollback copy: %v", err)
	}
}

// TestProgramPreviewsWriteNothing shows that install --dry-run creates neither
// the target nor its directory, and uninstall --dry-run keeps the program file
// and its rollback copy that it names.
func TestProgramPreviewsWriteNothing(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	writeFile(t, app.Executable, []byte("program"), 0o700)
	if err := cli.Execute(app, []string{"self", "install", "--dry-run", "--json"}); err != nil {
		t.Fatalf("install preview failed: %v\n%s", err, out.String())
	}
	if _, err := os.Stat(filepath.Dir(app.InstallTarget)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("install preview created the target directory: %v", err)
	}
	installed := filepath.Join(t.TempDir(), "aigw")
	writeFile(t, installed, []byte("installed"), 0o700)
	writeFile(t, program.RollbackPath(installed), []byte("previous"), 0o700)
	out.Reset()
	if err := cli.Execute(app, []string{"self", "uninstall", "--path", installed, "--dry-run", "--json"}); err != nil {
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
	codexTarget := filepath.Join(app.Host.Home, "work", "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	configureUninstallClients(t, app, codexTarget)
	files := map[string][]byte{}
	for _, path := range []string{app.Config.Path(), codexTarget, app.Executable} {
		files[path] = readFile(t, path)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"self", "uninstall", "--dry-run", "--json"}); err != nil {
		t.Fatalf("uninstall preview failed: %v\n%s", err, out.String())
	}
	type clientFile struct {
		Client string `json:"client"`
		File   string `json:"file"`
		Action string `json:"action"`
	}
	var document struct {
		DryRun           bool         `json:"dry_run"`
		WithdrawnClients []string     `json:"withdrawn_clients"`
		Removed          []string     `json:"removed"`
		Files            []clientFile `json:"client_files"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("uninstall preview document = %s, %v", out.String(), err)
	}
	// The document names the Codex file relative to the home directory; the
	// withdrawal restores it.
	withdrawsCodex := slices.ContainsFunc(document.Files, func(file clientFile) bool {
		return file.Client == codex.ID && file.File == "~/work/codex/config.toml" && file.Action == "restore_external"
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

// TestRemovedSpellingsAreUnknownFlags shows that each spelling that 0.5.0
// hid now fails as an unknown flag, naming the command's help, before the
// command reads a Token or writes anything.
func TestRemovedSpellingsAreUnknownFlags(t *testing.T) {
	installed := filepath.Join(t.TempDir(), "aigw")
	writeFile(t, installed, []byte("installed"), 0o700)
	destination := filepath.Join(t.TempDir(), "aigw")
	for _, test := range []struct {
		args       []string
		path, flag string
	}{
		{[]string{"self", "install", "--target", destination}, "self install", "--target"},
		{[]string{"self", "uninstall", "--target", installed}, "self uninstall", "--target"},
	} {
		t.Run(test.path, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			want := "unknown option " + test.flag + "; run `aigw " + test.path + " --help`"
			if err := cli.Execute(app, test.args); err == nil || !strings.Contains(out.String(), want) {
				t.Fatalf("%v: %v\n%s", test.args, err, out.String())
			}
			if _, err := os.Stat(app.Config.Path()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%v wrote configuration: %v", test.args, err)
			}
			if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%v installed a program: %v", test.args, err)
			}
			if got := readFile(t, installed); string(got) != "installed" {
				t.Fatalf("%v removed the installed program", test.args)
			}
		})
	}
}
