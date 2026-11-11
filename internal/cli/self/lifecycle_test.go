package self

import (
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

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/program"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
)

// fakeUpdater records each transition that a command asks for and returns
// the results that a test gives it.
type fakeUpdater struct {
	update, candidate, rollback, preview program.Transition
	updateErr, candidateErr, rollbackErr error
	previewErr                           error
	calls                                []string
	archive                              program.CandidateArchive
	rollbackConfig                       []byte
}

func (u *fakeUpdater) Update(context.Context, string) (program.Transition, error) {
	u.calls = append(u.calls, "update")
	return u.update, u.updateErr
}

func (u *fakeUpdater) UpdateCandidate(_ context.Context, _ string, archive program.CandidateArchive) (program.Transition, error) {
	u.calls, u.archive = append(u.calls, "archive"), archive
	return u.candidate, u.candidateErr
}

func (u *fakeUpdater) Preview(context.Context, string) (program.Transition, error) {
	u.calls = append(u.calls, "preview")
	return u.preview, u.previewErr
}

func (u *fakeUpdater) PreviewCandidate(_ string, archive program.CandidateArchive) (program.Transition, error) {
	u.calls, u.archive = append(u.calls, "preview archive"), archive
	return u.preview, u.previewErr
}

func (u *fakeUpdater) Rollback(_ context.Context, config []byte, _ program.CredentialCustody) (program.Transition, error) {
	u.calls, u.rollbackConfig = append(u.calls, "rollback"), config
	return u.rollback, u.rollbackErr
}

// lifecycleRuntime is a portable program in a private folder with the fake
// updater, a configuration store beside it and a memory credential store.
func lifecycleRuntime(t *testing.T, updater *fakeUpdater) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "bin", "aigw")
	writeProgram(t, executable, "current")
	out := &bytes.Buffer{}
	runtime := invocation.Context{
		Clients: clients.Registry(), Executable: executable, InstallTarget: executable, Version: "0.1.0",
		Config: store.NewStore(filepath.Join(root, "config", "aigw.toml"), clients.Admission()), Secrets: secret.NewMemoryStore(), Out: out, Width: 120,
	}
	if updater != nil {
		runtime.Updater = updater
	}
	return runtime, out
}

func writeProgram(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

// homebrewProgram makes runtime's program one that a Homebrew receipt owns.
func homebrewProgram(t *testing.T, runtime *invocation.Context) {
	t.Helper()
	cask := filepath.Join(t.TempDir(), "Caskroom", "aigw")
	runtime.Executable = filepath.Join(cask, "0.4.0", "aigw")
	writeProgram(t, runtime.Executable, "managed")
	writeProgram(t, filepath.Join(cask, ".metadata", "INSTALL_RECEIPT.json"), `{"homebrew_version":"5.1.0","source":{"tap":"owner/tap"}}`)
}

// samePath reports whether got names the file that want names: by their
// cleaned paths, or, where the forms can differ, as a Windows short and long
// name do, through os.SameFile on files that exist.
func samePath(got, want string) bool {
	if filepath.Clean(got) == filepath.Clean(want) {
		return true
	}
	gotInfo, gotErr := os.Stat(got)
	wantInfo, wantErr := os.Stat(want)
	return gotErr == nil && wantErr == nil && os.SameFile(gotInfo, wantInfo)
}

func runSelf(runtime invocation.Context, args ...string) error {
	command := NewCommand(runtime)
	command.SetArgs(args)
	command.SilenceErrors, command.SilenceUsage = true, true
	return command.Execute()
}

func TestSelfGroupsTheLifecycleCommands(t *testing.T) {
	command := NewCommand(invocation.Context{Clients: clients.Registry()})
	var names []string
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	if command.Runnable() || !slices.Equal(names, []string{"install", "rollback", "status", "uninstall", "update"}) {
		t.Fatalf("self runs itself or groups %q", names)
	}
}

// TestUpdateReportsEachTransition updates from the release source and from a
// local archive, in JSON and in text, and refuses an empty archive path and
// a program that Homebrew owns.
func TestUpdateReportsEachTransition(t *testing.T) {
	updater := &fakeUpdater{update: program.Transition{State: "updated", Version: "0.2.0", Summary: "updated to v0.2.0"},
		candidate: program.Transition{State: "updated", Version: "0.2.0", Summary: "installed the archive"}}
	runtime, out := lifecycleRuntime(t, updater)
	if err := runSelf(runtime, "update", "--json"); err != nil || !strings.Contains(out.String(), `"schema_version": 2`) || !strings.Contains(out.String(), `"next_action": "aigw sync"`) {
		t.Fatalf("update = %v\n%s", err, out)
	}
	out.Reset()
	if err := runSelf(runtime, "update", "--archive", "aigw.tar.gz", "--checksums", "checksums.txt"); err != nil || !strings.Contains(out.String(), "Verified local archive") ||
		updater.archive != (program.CandidateArchive{ArchivePath: "aigw.tar.gz", ChecksumsPath: "checksums.txt"}) {
		t.Fatalf("archive update = %v, %+v\n%s", err, updater.archive, out)
	}
	for _, args := range [][]string{{"update", "--archive", " ", "--checksums", "checksums.txt"}, {"update", "--archive", "aigw.tar.gz"}} {
		if err := runSelf(runtime, args...); err == nil {
			t.Fatalf("%v passed", args)
		}
	}
	if !slices.Equal(updater.calls, []string{"update", "archive"}) {
		t.Fatalf("calls = %q", updater.calls)
	}
	homebrewProgram(t, &runtime)
	if problem := terminal.ProblemFor(runSelf(runtime, "update")); problem.Fix != "brew upgrade --cask aigw" {
		t.Fatalf("a Homebrew update = %+v", problem)
	}
	runtime, _ = lifecycleRuntime(t, nil)
	if err := runSelf(runtime, "update"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("an update without an updater = %v", err)
	}
}

// TestUpdateNamesTheFixOfEachFailure names a local archive when the release
// sources failed, the program report otherwise, the problem of an archive
// with other bytes or a program that does not start, and a cleanup that did
// not finish after the replacement, in JSON and in text.
func TestUpdateNamesTheFixOfEachFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		updater  *fakeUpdater
		args     []string
		want     string
		document bool
	}{
		{"release source", &fakeUpdater{updateErr: fmt.Errorf("metadata: %w", program.ErrReleaseSource)}, []string{"update"}, "aigw self update --archive <archive> --checksums <checksums>", false},
		{"archive", &fakeUpdater{candidateErr: fmt.Errorf("checksum: %w", program.ErrReleaseSource)}, []string{"update", "--archive", "a", "--checksums", "c"}, "aigw self status", false},
		{"identity", &fakeUpdater{candidateErr: program.ErrCandidateIdentity}, []string{"update", "--archive", "a", "--checksums", "c"}, "Use the exact current release archive", false},
		{"startup", &fakeUpdater{candidateErr: program.ErrProgramStartupVerification}, []string{"update", "--archive", "a", "--checksums", "c"}, "Verify the archive for this platform", false},
		{"cleanup", &fakeUpdater{update: program.Transition{State: "updated", Version: "0.2.0"}, updateErr: errors.New("remove workspace: busy")}, []string{"update"}, "aigw self status", false},
		{"cleanup document", &fakeUpdater{update: program.Transition{State: "updated", Version: "0.2.0"}, updateErr: errors.New("remove workspace: busy")}, []string{"update", "--json"}, "Program updated; cleanup is incomplete", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, out := lifecycleRuntime(t, test.updater)
			err := runSelf(runtime, test.args...)
			text := out.String()
			if !test.document {
				problem := terminal.ProblemFor(err)
				text = problem.Fix + " " + problem.Title
			}
			if err == nil || !strings.Contains(text, test.want) {
				t.Fatalf("%s = %v\n%s", test.name, err, text)
			}
		})
	}
	runtime, _ := lifecycleRuntime(t, &fakeUpdater{rollback: program.Transition{State: "restored"}, rollbackErr: errors.New("remove workspace: busy")})
	if problem := terminal.ProblemFor(runSelf(runtime, "rollback")); problem.Title != "Program restored; cleanup is incomplete" {
		t.Fatalf("an incomplete rollback cleanup = %+v", problem)
	}
}

// TestUpdatePreviewNamesWhatItWouldWrite previews an update from the release
// source and from an archive, in JSON and in text, and its failures, while
// it installs nothing.
func TestUpdatePreviewNamesWhatItWouldWrite(t *testing.T) {
	updater := &fakeUpdater{preview: program.Transition{State: program.TransitionAvailable, Version: "0.2.0", Summary: "would update to v0.2.0"}}
	runtime, out := lifecycleRuntime(t, updater)
	if err := runSelf(runtime, "update", "--dry-run", "--json"); err != nil || !strings.Contains(out.String(), `"rollback": "`) || !strings.Contains(out.String(), "with this same program") {
		t.Fatalf("preview = %v\n%s", err, out)
	}
	out.Reset()
	if err := runSelf(runtime, "update", "--dry-run", "--archive", "a.tar.gz", "--checksums", "c.txt"); err != nil || !strings.Contains(out.String(), "Rollback copy") || !strings.Contains(out.String(), "would update to v0.2.0") {
		t.Fatalf("archive preview = %v\n%s", err, out)
	}
	updater.preview = program.Transition{State: program.TransitionCurrent, Version: "0.1.0", Summary: "already current"}
	out.Reset()
	if err := runSelf(runtime, "update", "--dry-run"); err != nil || strings.Contains(out.String(), "Rollback copy") || !strings.Contains(out.String(), "aigw sync") {
		t.Fatalf("a current preview = %v\n%s", err, out)
	}
	updater.previewErr = fmt.Errorf("metadata: %w", program.ErrReleaseSource)
	if problem := terminal.ProblemFor(runSelf(runtime, "update", "--dry-run")); problem.Fix != "aigw self update --dry-run --archive <archive> --checksums <checksums>" {
		t.Fatalf("a preview without the release source = %+v", problem)
	}
	updater.previewErr = errors.New("archive name has no version")
	if problem := terminal.ProblemFor(runSelf(runtime, "update", "--dry-run", "--archive", "a.tar.gz", "--checksums", "c.txt")); problem.Fix != "aigw self status" {
		t.Fatalf("a failed archive preview = %+v", problem)
	}
	if slices.Contains(updater.calls, "update") || slices.Contains(updater.calls, "archive") {
		t.Fatalf("a preview installed: %q", updater.calls)
	}
}

// TestRollbackRestoresTheRetainedProgram rolls back with the configuration
// that the retained program reads, in JSON and in text, and names the fix of
// each failure.
func TestRollbackRestoresTheRetainedProgram(t *testing.T) {
	updater := &fakeUpdater{rollback: program.Transition{State: "restored", Version: "0.0.9", Summary: "restored the previous program version."}}
	runtime, out := lifecycleRuntime(t, updater)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "claude-test")] = domain.Model{Provider: "team", Family: "claude-test", ID: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "claude-test"), "")
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runSelf(runtime, "rollback", "--json"); err != nil || !strings.Contains(out.String(), `"state": "restored"`) || len(updater.rollbackConfig) == 0 {
		t.Fatalf("rollback = %v, configuration %d bytes\n%s", err, len(updater.rollbackConfig), out)
	}
	out.Reset()
	if err := runSelf(runtime, "rollback"); err != nil || !strings.Contains(out.String(), "Program rollback") {
		t.Fatalf("rollback = %v\n%s", err, out)
	}
	updater.rollback = program.Transition{}
	updater.rollbackErr = fmt.Errorf("check: %w", program.ErrRollbackConfiguration)
	if problem := terminal.ProblemFor(runSelf(runtime, "rollback")); !strings.Contains(problem.Fix, "aigw config rollback") {
		t.Fatalf("an incompatible configuration = %+v", problem)
	}
	updater.rollbackErr = errors.New("activate: refused")
	if problem := terminal.ProblemFor(runSelf(runtime, "rollback")); problem.Fix != "aigw self status" {
		t.Fatalf("a failed rollback = %+v", problem)
	}
	runtime, _ = lifecycleRuntime(t, nil)
	if err := runSelf(runtime, "rollback"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("a rollback without an updater = %v", err)
	}
	runtime, _ = lifecycleRuntime(t, &fakeUpdater{})
	if err := os.MkdirAll(runtime.Config.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := runSelf(runtime, "rollback"); err == nil || !strings.Contains(err.Error(), "read configuration before program rollback") {
		t.Fatalf("an unreadable configuration = %v", err)
	}
}

// TestRollbackPreviewNamesTheRetainedProgram previews the retained program,
// its unread version and the configuration file, in JSON and in text, and
// refuses when nothing is retained.
func TestRollbackPreviewNamesTheRetainedProgram(t *testing.T) {
	updater := &fakeUpdater{}
	runtime, out := lifecycleRuntime(t, updater)
	if problem := terminal.ProblemFor(runSelf(runtime, "rollback", "--dry-run")); problem.Title != "No previous program to roll back to" || problem.Fix != "aigw self status" {
		t.Fatalf("a preview without a retained program = %+v", problem)
	}
	writeProgram(t, program.RollbackPath(runtime.Executable), "previous")
	if err := runSelf(runtime, "rollback", "--dry-run", "--json"); err != nil || !strings.Contains(out.String(), `"version_unread": "its build information cannot be read"`) ||
		!strings.Contains(out.String(), `"configuration": `) {
		t.Fatalf("preview = %v\n%s", err, out)
	}
	out.Reset()
	if err := runSelf(runtime, "rollback", "--dry-run"); err != nil || !strings.Contains(out.String(), "Unknown: its build information cannot be read") {
		t.Fatalf("preview = %v\n%s", err, out)
	}
	if len(updater.calls) != 0 {
		t.Fatalf("a preview rolled back: %q", updater.calls)
	}
}

// TestHomebrewRollbackPreparesTheCredentials returns the credentials for an
// earlier Homebrew release, in JSON and in text, and its preview changes
// nothing; a configuration that does not load stops it with its fix.
func TestHomebrewRollbackPreparesTheCredentials(t *testing.T) {
	runtime, out := lifecycleRuntime(t, &fakeUpdater{})
	homebrewProgram(t, &runtime)
	for _, args := range [][]string{{"rollback", "--dry-run", "--json"}, {"rollback", "--dry-run"}} {
		out.Reset()
		if err := runSelf(runtime, args...); err != nil || !strings.Contains(out.String(), "aigw self rollback") {
			t.Fatalf("%v = %v\n%s", args, err, out)
		}
	}
	for _, args := range [][]string{{"rollback", "--json"}, {"rollback"}} {
		out.Reset()
		if err := runSelf(runtime, args...); err != nil || !strings.Contains(out.String(), "Credentials are ready for an earlier AIGW release") {
			t.Fatalf("%v = %v\n%s", args, err, out)
		}
	}
	writeProgram(t, runtime.Config.Path(), "not = [toml")
	if problem := terminal.ProblemFor(runSelf(runtime, "rollback")); !strings.Contains(problem.Fix, "Then run `aigw self rollback` again") {
		t.Fatalf("an unreadable configuration = %+v", problem)
	}
}

// TestInstallAndUninstallPreviewAndRefuse previews an installation over
// another program and an uninstallation of a configured home, and refuses a
// folder as the target with the program file inside it.
func TestInstallAndUninstallPreviewAndRefuse(t *testing.T) {
	runtime, out := lifecycleRuntime(t, nil)
	other := filepath.Join(t.TempDir(), "aigw")
	writeProgram(t, other, "another program")
	if err := runSelf(runtime, "install", "--path", other, "--dry-run"); err != nil || !strings.Contains(out.String(), "Rollback copy") {
		t.Fatalf("install preview = %v\n%s", err, out)
	}
	folder := t.TempDir()
	if problem := terminal.ProblemFor(runSelf(runtime, "install", "--path", folder)); !strings.Contains(problem.Fix, filepath.Join(folder, "aigw")) {
		t.Fatalf("a folder as the target = %+v", problem)
	}
	cfg := domain.NewConfig(clients.Admission())
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	// The document names the program in its fields, where JSON escapes every
	// backslash of a Windows path, and the text names it in its Remove row.
	out.Reset()
	if err := runSelf(runtime, "uninstall", "--dry-run", "--json"); err != nil {
		t.Fatalf("uninstall --dry-run --json = %v\n%s", err, out)
	}
	var document struct {
		Path    string   `json:"path"`
		Removed []string `json:"removed"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil || !samePath(document.Path, runtime.Executable) ||
		len(document.Removed) != 1 || !samePath(document.Removed[0], runtime.Executable) {
		t.Fatalf("uninstall --dry-run --json names %+v, not %s: %v\n%s", document, runtime.Executable, err, out)
	}
	var row bytes.Buffer
	terminal.NewRendererWithWidth(&row, runtime.Color, runtime.Width).Rows(terminal.Field{Label: "Remove", Value: runtime.Executable})
	out.Reset()
	if err := runSelf(runtime, "uninstall", "--dry-run"); err != nil || !strings.Contains(out.String(), row.String()) {
		t.Fatalf("uninstall --dry-run = %v, without the row\n%s\n%s", err, row.String(), out)
	}
	if _, err := os.Stat(runtime.Executable); err != nil {
		t.Fatalf("a preview removed the program: %v", err)
	}
	if problem := terminal.ProblemFor(runSelf(runtime, "uninstall", "--path", folder)); !strings.Contains(problem.Fix, "naming the program file inside "+folder) {
		t.Fatalf("a folder as the uninstall target = %+v", problem)
	}
}

// TestLifecycleRefusalsNameTheirOwnFix covers the refusals that state a fix
// of their own: an update error that names one, a target that installing
// cannot read the running program for, a folder whose program file is a
// folder too, no installation target at all, and a Homebrew program that
// uninstalling leaves to Homebrew.
func TestLifecycleRefusalsNameTheirOwnFix(t *testing.T) {
	stated := terminal.ProblemError("Release source is busy", "", "", "Wait for the release source, then run the same command again.", errors.New("busy"))
	runtime, _ := lifecycleRuntime(t, &fakeUpdater{updateErr: stated})
	if problem := terminal.ProblemFor(runSelf(runtime, "update")); problem.Title != "Release source is busy" {
		t.Fatalf("an error with its own fix = %+v", problem)
	}
	missing, _ := lifecycleRuntime(t, nil)
	missing.Executable = filepath.Join(t.TempDir(), "gone")
	if err := runSelf(missing, "install", "--path", filepath.Join(t.TempDir(), "aigw")); err == nil {
		t.Fatal("installing a program that cannot be read passed")
	}
	nested := t.TempDir()
	if err := os.MkdirAll(filepath.Join(nested, "aigw"), 0o700); err != nil {
		t.Fatal(err)
	}
	if problem := terminal.ProblemFor(runSelf(runtime, "install", "--path", nested)); !strings.Contains(problem.Fix, "is a directory too") {
		t.Fatalf("a folder whose program file is a folder = %+v", problem)
	}
	untargeted, _ := lifecycleRuntime(t, nil)
	untargeted.InstallTarget = ""
	if err := runSelf(untargeted, "install"); err == nil || !strings.Contains(err.Error(), "name the program file with --path") {
		t.Fatalf("an installation without a target = %v", err)
	}
	homebrewProgram(t, &runtime)
	if problem := terminal.ProblemFor(runSelf(runtime, "uninstall")); problem.Fix != "brew uninstall --cask aigw" {
		t.Fatalf("a Homebrew uninstall = %+v", problem)
	}
	if problem := terminal.ProblemFor(runSelf(runtime, "install", "--path", runtime.Executable)); problem.Fix != "brew upgrade --cask aigw" {
		t.Fatalf("installing over a Homebrew program = %+v", problem)
	}
}

// TestUninstallThatCannotRemoveTheProgramSaysWhatChanged keeps a program
// that its read-only folder protects: a configured home's document says that
// the client settings went while the program stayed, and a home without a
// configuration says that no client file changed. A preview of an absent
// program says that there is none.
func TestUninstallThatCannotRemoveTheProgramSaysWhatChanged(t *testing.T) {
	runtime, out := lifecycleRuntime(t, nil)
	if err := runSelf(runtime, "uninstall", "--path", filepath.Join(t.TempDir(), "aigw"), "--dry-run"); err != nil || !strings.Contains(out.String(), "No program file at") {
		t.Fatalf("an absent program's preview = %v\n%s", err, out)
	}
	if goruntime := os.Getenv("GOOS"); goruntime == "windows" || filepath.Separator == '\\' {
		t.Skip("a read-only folder does not keep a file on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("folder permissions do not bind root")
	}
	for _, configured := range []bool{true, false} {
		runtime, out := lifecycleRuntime(t, nil)
		if configured {
			if err := runtime.Config.Save(domain.NewConfig(clients.Admission())); err != nil {
				t.Fatal(err)
			}
		}
		folder := filepath.Dir(runtime.Executable)
		if err := os.Chmod(folder, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })
		err := runSelf(runtime, "uninstall", "--json")
		want := "Portable program removal is incomplete"
		if configured {
			want = `"problem": "Client settings were removed; program removal is incomplete"`
		} else {
			out.WriteString(terminal.ProblemFor(err).Title)
		}
		if err == nil || !strings.Contains(out.String(), want) {
			t.Fatalf("configured %v: %v\n%s", configured, err, out)
		}
	}
}

// TestInstallAndUninstallReportTheirResultAsJSON installs a copy as JSON,
// previews installing it again, which only makes the copy executable, names
// a folder whose path needs quoting in prose, and removes the copy as JSON.
func TestInstallAndUninstallReportTheirResultAsJSON(t *testing.T) {
	runtime, out := lifecycleRuntime(t, nil)
	copied := filepath.Join(t.TempDir(), "copy", "aigw")
	if err := runSelf(runtime, "install", "--path", copied, "--json"); err != nil || !strings.Contains(out.String(), `"schema_version": 2`) {
		t.Fatalf("install = %v\n%s", err, out)
	}
	out.Reset()
	if err := runSelf(runtime, "install", "--path", copied, "--dry-run"); err != nil || !strings.Contains(out.String(), "Already this program") {
		t.Fatalf("installing onto the same program = %v\n%s", err, out)
	}
	spaced := filepath.Join(t.TempDir(), "with space")
	if err := os.MkdirAll(spaced, 0o700); err != nil {
		t.Fatal(err)
	}
	if problem := terminal.ProblemFor(runSelf(runtime, "install", "--path", spaced)); !strings.Contains(problem.Fix, "naming the program file "+filepath.Join(spaced, "aigw")) {
		t.Fatalf("a folder that needs quoting = %+v", problem)
	}
	out.Reset()
	if err := runSelf(runtime, "uninstall", "--path", copied, "--json"); err != nil || !strings.Contains(out.String(), `"program_found": true`) {
		t.Fatalf("uninstall = %v\n%s", err, out)
	}
}

// TestInstallFailureNamesTheProgramFileOfAFolder gives a folder that the
// installation itself refused, as a folder that appeared after the argument
// check would be, the fix that names the program file inside it.
func TestInstallFailureNamesTheProgramFileOfAFolder(t *testing.T) {
	runtime, _ := lifecycleRuntime(t, nil)
	command := newInstallCommand(runtime)
	folder := t.TempDir()
	problem := terminal.ProblemFor(installFailure(command, runtime, folder, &program.TargetDirectoryError{Path: folder}))
	if !strings.Contains(problem.Fix, filepath.Join(folder, "aigw")) {
		t.Fatalf("a folder that the installation refused = %+v", problem)
	}
}
