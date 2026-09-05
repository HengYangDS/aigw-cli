package cli_test

import (
	"errors"
	"strings"
	"testing"
)

func TestUpdateWithoutUpdaterIsRejected(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Updater = nil
	if err := execute(t, app, "update"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateCandidateUsesExplicitOfflineInputs(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	updater := &fakeUpdater{candidateResult: "updated to v0.2.0 from a verified local candidate"}
	app.Updater = updater
	if err := execute(t, app, "update", "--candidate", "/tmp/aigw_0.2.0_darwin_arm64.tar.gz", "--checksums", "/tmp/checksums.txt"); err != nil {
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
}

func TestUpdatePreservesReleaseFailureAsItsCause(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cause := errors.New("release lookup failed")
	app.Updater = &fakeUpdater{updateErr: cause}

	if err := execute(t, app, "update"); !errors.Is(err, cause) {
		t.Fatalf("error = %v, want %v", err, cause)
	}
}
