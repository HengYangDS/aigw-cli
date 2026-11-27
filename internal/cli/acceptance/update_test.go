package cli_test

import (
	"aigw-cli/internal/cli"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/program"
	"aigw-cli/internal/store"
)

func TestUpdateCandidateRequiresChecksumManifest(t *testing.T) {
	t.Parallel()
	app, _, _, _, _ := testApp(t, "")
	app.Updater = &fakeUpdater{}
	err := cli.Execute(app, []string{"self", "update", "--archive", "/tmp/aigw_0.2.0_darwin_arm64.tar.gz"})
	if err == nil || !strings.Contains(err.Error(), "must all be set") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateInvalidInputsLeaveOperationAndConfigurationUntouched(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--archive", ""},
		{"--archive", "", "--checksums", ""},
		{"--archive", " ", "--checksums", "checksums.txt"},
		{"--archive", "archive.tar.gz", "--checksums", "\t"},
		{"--archive", "archive.tar.gz"},
		{"--checksums", "checksums.txt"},
		{"--rollback", "--archive", "archive.tar.gz", "--checksums", "checksums.txt"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			root := filepath.Join(t.TempDir(), "unconfigured")
			app.Config = store.NewStore(filepath.Join(root, "aigw.toml"), clients.Admission())
			updater := &fakeUpdater{}
			app.Updater = updater
			err := cli.Execute(app, append([]string{"self", "update"}, args...))
			if err == nil || updater.updateCalls != 0 || updater.candidateCalls != 0 || updater.rollbackCalls != 0 {
				t.Fatalf("error=%v online=%d candidate=%d rollback=%d", err, updater.updateCalls, updater.candidateCalls, updater.rollbackCalls)
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid invocation created configuration state: %v", err)
			}
		})
	}
}

func TestUpdateRollbackUsesLocalProgramRollbackOnly(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{rollbackResult: restored("restored the previous program version and retained the replaced program.")}
	app.Updater = updater
	if err := cli.Execute(app, []string{"self", "rollback"}); err != nil {
		t.Fatal(err)
	}
	if updater.rollbackCalls != 1 || updater.updateCalls != 0 {
		t.Fatalf("update calls=%d rollback calls=%d", updater.updateCalls, updater.rollbackCalls)
	}
	if !strings.Contains(out.String(), "Program rollback") || !strings.Contains(out.String(), "restored the previous program version") {
		t.Fatalf("output = %s", out.String())
	}
	if !strings.Contains(out.String(), "aigw sync") {
		t.Fatalf("program rollback omitted client reconciliation: %s", out.String())
	}
}

func TestUpdateWithoutRollbackKeepsNetworkUpdatePath(t *testing.T) {
	t.Parallel()
	app, _, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{updateResult: updated("0.2.0", "updated to v0.2.0.")}
	app.Updater = updater
	if err := cli.Execute(app, []string{"self", "update"}); err != nil {
		t.Fatal(err)
	}
	if updater.updateCalls != 1 || updater.rollbackCalls != 0 {
		t.Fatalf("update calls=%d rollback calls=%d", updater.updateCalls, updater.rollbackCalls)
	}
}

func TestUpdateRollbackReturnsLocalRollbackError(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := testApp(t, "")
	cause := errors.New("read retained predecessor: permission denied")
	app.Updater = &fakeUpdater{rollbackErr: cause}
	err := cli.Execute(app, []string{"self", "rollback"})
	if err == nil || err.Error() != "Program rollback did not complete" || !errors.Is(err, cause) {
		t.Fatalf("error = %v", err)
	}
	for _, want := range []string{
		"Program rollback did not complete",
		"AIGW could not activate the retained previous program.",
		"No previous program version was confirmed active.",
		"Fix\n  aigw self status",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), cause.Error()) {
		t.Fatalf("output exposes implementation error:\n%s", out.String())
	}
	if count := strings.Count(out.String(), "aigw self status"); count != 1 || strings.Contains(out.String(), "aigw check") {
		t.Fatalf("safe next action count = %d, want 1 and no client check:\n%s", count, out.String())
	}
}

func TestUpdateRollbackExecutionFailureKeepsRollbackContext(t *testing.T) {
	t.Parallel()
	privatePath := filepath.Join(t.TempDir(), "previous")
	permission := &os.PathError{Op: "fork/exec", Path: privatePath, Err: os.ErrPermission}
	for _, test := range []struct {
		name    string
		failure error
	}{
		{name: "startup verification", failure: fmt.Errorf("%w: %w", program.ErrProgramStartupVerification, permission)},
		{name: "configuration export permission", failure: fmt.Errorf("retained program configuration export failed: %w", permission)},
		{name: "configuration export deadline", failure: fmt.Errorf("retained program configuration export failed: %w", context.DeadlineExceeded)},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			app.Updater = &fakeUpdater{rollbackErr: test.failure}
			err := cli.Execute(app, []string{"self", "rollback"})
			if !errors.Is(err, test.failure) {
				t.Fatalf("rollback failure lost its typed cause: %v", err)
			}
			if !strings.Contains(out.String(), "Program rollback did not complete") ||
				strings.Contains(out.String(), "incompatible") || strings.Contains(out.String(), "Candidate program") || strings.Contains(out.String(), privatePath) {
				t.Fatalf("rollback failure lost its safe operation context: %s", out.String())
			}
		})
	}
}

func TestUpdateRollbackPreservesExactConfigurationOnIncompatibility(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := testApp(t, "")
	config := []byte("version = 3\n# Preserve exact user bytes.\n[recommended_routes]\nclaude = 'team'\n")
	if err := os.WriteFile(app.Config.Path(), config, 0o600); err != nil {
		t.Fatal(err)
	}
	updater := &fakeUpdater{rollbackErr: program.ErrRollbackConfiguration}
	app.Updater = updater
	err := cli.Execute(app, []string{"self", "rollback"})
	if !errors.Is(err, program.ErrRollbackConfiguration) || string(updater.rollbackConfig) != string(config) || updater.rollbackCalls != 1 {
		t.Fatalf("rollback input or cause changed: %q, %v", updater.rollbackConfig, err)
	}
	if actual, err := os.ReadFile(app.Config.Path()); err != nil || string(actual) != string(config) {
		t.Fatalf("rollback changed config: %q, %v", actual, err)
	}
	for _, want := range []string{"Program rollback did not complete", "configuration-read check", "remain unchanged", "compatible program", "copy of aigw.toml kept before a schema change", "aigw config rollback", "do not delete policy fields by hand"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rollback omitted %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "incompatible") {
		t.Fatalf("reader refusal inferred an unproved cause: %s", out.String())
	}
}

func TestUpdateRollbackStopsBeforeUpdaterWhenConfigurationCannotBeRead(t *testing.T) {
	t.Parallel()
	app, _, _, _, _ := testApp(t, "")
	if err := os.Mkdir(app.Config.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	updater := &fakeUpdater{}
	app.Updater = updater
	if err := cli.Execute(app, []string{"self", "rollback"}); err == nil || updater.rollbackCalls != 0 {
		t.Fatalf("rollback ignored unreadable configuration: %v, calls=%d", err, updater.rollbackCalls)
	}
}

// homebrewProgram makes app's executable the program of a Homebrew cask.
func homebrewProgram(t *testing.T, app *cli.App) {
	t.Helper()
	cask := filepath.Join(t.TempDir(), "Caskroom", "aigw")
	app.Executable = filepath.Join(cask, "0.4.0", "aigw")
	receipt := filepath.Join(cask, ".metadata", "INSTALL_RECEIPT.json")
	if err := os.MkdirAll(filepath.Dir(receipt), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receipt, []byte(`{"homebrew_version":"5.1.0","source":{"tap":"hengyangds/tap"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestSelfUpdateAndRollbackHelpDescribeTheProgramTransition: update's help
// says what an update changes and what follows it, and rollback's says what
// it restores and what its check stages. Each names what Homebrew does for a
// program that it installed, where the command is refused.
func TestSelfUpdateAndRollbackHelpDescribeTheProgramTransition(t *testing.T) {
	t.Parallel()
	for command, wants := range map[string][]string{
		"update": {
			"Homebrew upgrades a program that it installed",
			"Client settings and Tokens do not change",
			"Run aigw sync, then aigw check, before resuming clients",
			"--archive installs a local release archive, verified against --checksums",
		},
		"rollback": {
			"Activate the program that the last update or installation retained",
			"gives the previous program the current configuration",
			"a configuration that it cannot read blocks the rollback without changing files",
			"Homebrew installs an earlier release of a program that it installed",
		},
	} {
		app, out, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"self", command, "--help"}); err != nil {
			t.Fatal(err)
		}
		help := strings.Join(strings.Fields(out.String()), " ")
		for _, want := range wants {
			if !strings.Contains(help, want) {
				t.Fatalf("self %s help omitted %q: %s", command, want, out.String())
			}
		}
	}
}

func TestUpdateWithoutUpdaterIsRejected(t *testing.T) {
	t.Parallel()
	app, _, _, _, _ := testApp(t, "")
	app.Updater = nil
	if err := cli.Execute(app, []string{"self", "update"}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateCandidateUsesExplicitOfflineInputs(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{candidateResult: updated("0.2.0", "updated to v0.2.0 from a verified local candidate")}
	app.Updater = updater
	if err := cli.Execute(app, []string{"self", "update", "--archive", "/tmp/aigw_0.2.0_darwin_arm64.tar.gz", "--checksums", "/tmp/checksums.txt"}); err != nil {
		t.Fatal(err)
	}
	if updater.candidateCalls != 1 || updater.updateCalls != 0 || updater.rollbackCalls != 0 {
		t.Fatalf("network=%d candidate=%d rollback=%d", updater.updateCalls, updater.candidateCalls, updater.rollbackCalls)
	}
	if updater.candidateReceived.ArchivePath != "/tmp/aigw_0.2.0_darwin_arm64.tar.gz" || updater.candidateReceived.ChecksumsPath != "/tmp/checksums.txt" {
		t.Fatalf("candidate = %#v", updater.candidateReceived)
	}
	if !strings.Contains(out.String(), "Verified local archive") {
		t.Fatalf("output = %s", out.String())
	}
	if !strings.Contains(out.String(), "aigw sync") {
		t.Fatalf("program update omitted client reconciliation: %s", out.String())
	}
}

func TestUpdateCandidateStartupFailureKeepsSafeDiagnosis(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := testApp(t, "")
	privatePath := filepath.Join(t.TempDir(), "candidate")
	cause := &os.PathError{Op: "fork/exec", Path: privatePath, Err: os.ErrPermission}
	app.Updater = &fakeUpdater{candidateErr: fmt.Errorf("%w: %w", program.ErrProgramStartupVerification, cause)}

	err := cli.Execute(app, []string{"self", "update", "--archive", "candidate.tar.gz", "--checksums", "checksums.txt"})
	if !errors.Is(err, program.ErrProgramStartupVerification) || !errors.Is(err, cause) {
		t.Fatalf("candidate failure lost its typed cause: %v", err)
	}
	for _, want := range []string{"The archive's program failed its startup check", "installed program is unchanged", "archive for this platform"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("candidate diagnosis lacks %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), privatePath) || strings.Contains(out.String(), "Local file access failed") {
		t.Fatalf("candidate diagnosis exposed the path or obscured the failure: %s", out.String())
	}
}

func TestUpdatePreservesReleaseFailureAsItsCause(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := testApp(t, "")
	const canary = "TRACEBACK_CANARY /private/forge-only-canary"
	cause := fmt.Errorf("release lookup failed: %w: %w: %s", program.ErrReleaseSource, errors.New("exit status 1"), canary)
	app.Updater = &fakeUpdater{updateErr: cause}

	if err := cli.Execute(app, []string{"self", "update"}); !errors.Is(err, cause) {
		t.Fatalf("error = %v, want %v", err, cause)
	}
	if strings.Contains(out.String(), canary) || !strings.Contains(out.String(), "Fix\n  aigw self update --archive <archive> --checksums <checksums>") ||
		strings.Contains(out.String(), "aigw check --depth local") {
		t.Fatalf("update exposed private diagnostics or omitted recovery: %s", out.String())
	}
}

func TestUpdateReportsCompletedReplacementWithIncompleteCleanup(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		args            []string
		updater         *fakeUpdater
		title, impact   string
		forbiddenImpact string
	}{
		{
			args:    []string{"self", "update"},
			updater: &fakeUpdater{updateResult: updated("1.0.0", "updated to v1.0.0 verified from GitHub")},
			title:   "Program updated", impact: "client settings and credentials were not changed",
		},
		{
			args:    []string{"self", "rollback"},
			updater: &fakeUpdater{rollbackResult: restored("restored the previous program version.")},
			title:   "Program restored", impact: "The previous program is active and reads its credentials",
			forbiddenImpact: "credentials were not changed",
		},
	} {
		t.Run(test.title, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			cause := &os.PathError{Op: "remove", Path: "/private/update-only-canary", Err: os.ErrPermission}
			test.updater.updateErr, test.updater.rollbackErr = cause, cause
			app.Updater = test.updater
			if err := cli.Execute(app, test.args); !errors.Is(err, cause) {
				t.Fatalf("completed transition lost cleanup failure: %v", err)
			}
			text := strings.Join(strings.Fields(out.String()), " ")
			if !strings.Contains(text, test.title) || !strings.Contains(text, "cleanup is incomplete") || !strings.Contains(text, test.impact) || strings.Contains(text, cause.Path) {
				t.Fatalf("completed transition has an incorrect or unsafe state: %s", out.String())
			}
			if test.forbiddenImpact != "" && strings.Contains(text, test.forbiddenImpact) {
				t.Fatalf("rollback claimed unchanged credentials: %s", out.String())
			}
		})
	}
}

func TestUpdateSameVersionIdentityConflictHasSafeDiagnosis(t *testing.T) {
	t.Parallel()
	app, out, _, _, _ := testApp(t, "")
	cause := program.ErrCandidateIdentity
	app.Updater = &fakeUpdater{candidateErr: cause}
	err := cli.Execute(app, []string{"self", "update", "--archive", "candidate.tar.gz", "--checksums", "checksums.txt"})
	if !errors.Is(err, cause) || !strings.Contains(out.String(), "other program bytes") || !strings.Contains(out.String(), "unchanged") {
		t.Fatalf("candidate identity failure lost its safe meaning: error=%v output=%s", err, out.String())
	}
}

// updated and restored are the transitions that a fake updater reports.
func updated(version, summary string) program.Transition {
	return program.Transition{State: program.TransitionUpdated, Version: version, Summary: summary}
}

func restored(summary string) program.Transition {
	return program.Transition{State: program.TransitionRestored, Version: "0.0.9", Summary: summary}
}

type fakeUpdater struct {
	updateCalls       int
	candidateCalls    int
	rollbackCalls     int
	updateResult      program.Transition
	candidateResult   program.Transition
	rollbackResult    program.Transition
	rollbackConfig    []byte
	updateErr         error
	candidateErr      error
	rollbackErr       error
	candidateReceived program.CandidateArchive
	previewCalls      int
	previewResult     program.Transition
	previewErr        error
}

func (u *fakeUpdater) Update(_ context.Context, _ string) (program.Transition, error) {
	u.updateCalls++
	return u.updateResult, u.updateErr
}

func (u *fakeUpdater) UpdateCandidate(_ context.Context, _ string, candidate program.CandidateArchive) (program.Transition, error) {
	u.candidateCalls++
	u.candidateReceived = candidate
	return u.candidateResult, u.candidateErr
}

func (u *fakeUpdater) Rollback(_ context.Context, config []byte) (program.Transition, error) {
	u.rollbackConfig = append([]byte(nil), config...)
	u.rollbackCalls++
	return u.rollbackResult, u.rollbackErr
}
