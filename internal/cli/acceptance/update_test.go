package cli_test

import (
	"aigw-cli/internal/cli"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/upgrade"
)

func TestUpdateCandidateRequiresChecksumManifest(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Updater = &fakeUpdater{}
	err := cli.Execute(app, []string{"update", "--candidate", "/tmp/aigw_0.2.0_darwin_arm64.tar.gz"})
	if err == nil || !strings.Contains(err.Error(), "must all be set") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateInvalidInputsLeaveOperationAndConfigurationUntouched(t *testing.T) {
	for _, args := range [][]string{
		{"--candidate", ""},
		{"--candidate", "", "--checksums", ""},
		{"--candidate", " ", "--checksums", "checksums.txt"},
		{"--candidate", "archive.zip", "--checksums", "\t"},
		{"--candidate", "archive.zip"},
		{"--checksums", "checksums.txt"},
		{"--rollback", "--candidate", "archive.zip", "--checksums", "checksums.txt"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			root := filepath.Join(t.TempDir(), "unconfigured")
			app.Config = configuration.NewStore(filepath.Join(root, "config.toml"))
			updater := &fakeUpdater{}
			app.Updater = updater
			err := cli.Execute(app, append([]string{"update"}, args...))
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
	app, out, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{rollbackResult: "restored the previous program version; you can run `aigw update --rollback` again to restore the current version."}
	app.Updater = updater
	if err := cli.Execute(app, []string{"update", "--rollback"}); err != nil {
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
	app, _, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{updateResult: "updated to v0.2.0."}
	app.Updater = updater
	if err := cli.Execute(app, []string{"update"}); err != nil {
		t.Fatal(err)
	}
	if updater.updateCalls != 1 || updater.rollbackCalls != 0 {
		t.Fatalf("update calls=%d rollback calls=%d", updater.updateCalls, updater.rollbackCalls)
	}
}

func TestUpdateRollbackReturnsLocalRollbackError(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cause := errors.New("read retained predecessor: permission denied")
	app.Updater = &fakeUpdater{rollbackErr: cause}
	err := cli.Execute(app, []string{"update", "--rollback"})
	if err == nil || err.Error() != "Program rollback did not complete" || !errors.Is(err, cause) {
		t.Fatalf("error = %v", err)
	}
	for _, want := range []string{
		"Program rollback did not complete",
		"AIGW could not activate the retained previous program.",
		"No previous program version was confirmed active.",
		"aigw check",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), cause.Error()) {
		t.Fatalf("output exposes implementation error:\n%s", out.String())
	}
	if count := strings.Count(out.String(), "aigw check"); count != 1 {
		t.Fatalf("safe next action count = %d, want 1:\n%s", count, out.String())
	}
}

func TestUpdateHelpDescribesOfflineProgramRollback(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{"update", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Roll back the portable AIGW program to the previous version offline") {
		t.Fatalf("help = %s", out.String())
	}
	if !strings.Contains(out.String(), "Before rollback, disable enabled client integrations") {
		t.Fatalf("help omitted rollback projection boundary: %s", out.String())
	}
}

func TestUpdateWithoutUpdaterIsRejected(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Updater = nil
	if err := cli.Execute(app, []string{"update"}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateCandidateUsesExplicitOfflineInputs(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{candidateResult: "updated to v0.2.0 from a verified local candidate"}
	app.Updater = updater
	if err := cli.Execute(app, []string{"update", "--candidate", "/tmp/aigw_0.2.0_darwin_arm64.tar.gz", "--checksums", "/tmp/checksums.txt"}); err != nil {
		t.Fatal(err)
	}
	if updater.candidateCalls != 1 || updater.updateCalls != 0 || updater.rollbackCalls != 0 {
		t.Fatalf("network=%d candidate=%d rollback=%d", updater.updateCalls, updater.candidateCalls, updater.rollbackCalls)
	}
	if updater.candidateReceived.ArchivePath != "/tmp/aigw_0.2.0_darwin_arm64.tar.gz" || updater.candidateReceived.ChecksumsPath != "/tmp/checksums.txt" {
		t.Fatalf("candidate = %#v", updater.candidateReceived)
	}
	if !strings.Contains(out.String(), "Verified local candidate") {
		t.Fatalf("output = %s", out.String())
	}
	if !strings.Contains(out.String(), "aigw sync") {
		t.Fatalf("program update omitted client reconciliation: %s", out.String())
	}
}

func TestUpdatePreservesReleaseFailureAsItsCause(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cause := errors.New("release lookup failed")
	app.Updater = &fakeUpdater{updateErr: cause}

	if err := cli.Execute(app, []string{"update"}); !errors.Is(err, cause) {
		t.Fatalf("error = %v, want %v", err, cause)
	}
}

type fakeUpdater struct {
	updateCalls       int
	candidateCalls    int
	rollbackCalls     int
	updateResult      string
	candidateResult   string
	rollbackResult    string
	updateErr         error
	candidateErr      error
	rollbackErr       error
	candidateReceived upgrade.CandidateArchive
}

func (u *fakeUpdater) Update(_ context.Context, _ string) (string, error) {
	u.updateCalls++
	return u.updateResult, u.updateErr
}

func (u *fakeUpdater) UpdateCandidate(_ context.Context, _ string, candidate upgrade.CandidateArchive) (string, error) {
	u.candidateCalls++
	u.candidateReceived = candidate
	return u.candidateResult, u.candidateErr
}

func (u *fakeUpdater) Rollback(_ context.Context) (string, error) {
	u.rollbackCalls++
	return u.rollbackResult, u.rollbackErr
}
