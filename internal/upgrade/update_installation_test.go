package upgrade_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/upgrade"
)

func TestUpdateRejectsDuplicateChecksumEntries(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n%x  ./%s\n", sum, name, sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "duplicate checksum") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary replaced after duplicate checksum entry: %q", got)
	}
}

func TestUpdateRejectsArchiveWithMultipleBinaries(t *testing.T) {
	archive := tarGzEntries(t,
		[]byte("aigw_0.2.0_darwin_arm64/aigw"), []byte("first"),
		[]byte("aigw_0.2.0_darwin_arm64/aigw"), []byte("second"),
	)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n", sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "multiple expected AIGW binaries") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary replaced after ambiguous archive: %q", got)
	}
}

func TestUpdateDownloadsVerifiesAndAtomicallyReplacesBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{
		archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, name),
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "new-binary" || !strings.Contains(message, "v0.2.0") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
	backup, err := os.ReadFile(filepath.Join(filepath.Dir(binary), ".aigw.previous"))
	if err != nil {
		t.Fatalf("read rollback binary: %v", err)
	}
	if string(backup) != "old-binary" {
		t.Fatalf("rollback binary=%q, want old-binary", backup)
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(binary), ".aigw.previous"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("rollback mode = %o, want 755", info.Mode().Perm())
	}
}

func TestUpdateReplacesOnlyTheSinglePreviousRollbackBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(filepath.Dir(binary), ".aigw.previous")
	if err := os.WriteFile(backupPath, []byte("older-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != "current-binary" {
		t.Fatalf("rollback binary=%q, want immediate prior binary", backup)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(binary), ".aigw.previous.previous")); !os.IsNotExist(err) {
		t.Fatalf("unexpected chained rollback binary: %v", err)
	}
}

func TestUpdateMakesReplacedRollbackBinaryExecutable(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(filepath.Dir(binary), ".aigw.previous")
	if err := os.WriteFile(backupPath, []byte("stale-rollback"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("rollback mode = %o, want 755", info.Mode().Perm())
	}
}

func TestPortableRollbackSwapsCurrentAndPreviousBinary(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "aigw")
	backup := filepath.Join(filepath.Dir(binary), ".aigw.previous")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("previous-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: upgrade.ChannelPortable, Executable: binary}
	message, err := u.Rollback(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "restored the previous program version") {
		t.Fatalf("message = %q", message)
	}
	current, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "previous-binary" || string(previous) != "current-binary" {
		t.Fatalf("current=%q previous=%q", current, previous)
	}
	for _, path := range []string{filepath.Join(filepath.Dir(binary), ".aigw.rollback.stage"), filepath.Join(filepath.Dir(binary), ".aigw.previous.rollback")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("rollback left staging residue %s: %v", path, err)
		}
	}
}

func TestPortableRollbackRefusesMissingPreviousBinary(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: upgrade.ChannelPortable, Executable: binary}
	_, err := u.Rollback(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no previous portable AIGW binary") {
		t.Fatalf("error = %v", err)
	}
}

func TestPortableRollbackRoundTripsProgramBinaries(t *testing.T) {
	dir := t.TempDir()
	currentSource := filepath.Join(dir, "current-source")
	previousSource := filepath.Join(dir, "previous-source")
	current := filepath.Join(dir, "aigw")
	previous := filepath.Join(dir, ".aigw.previous")
	if err := os.WriteFile(currentSource, []byte("current-program"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(previousSource, []byte("previous-program"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFixtureFile(currentSource, current); err != nil {
		t.Fatal(err)
	}
	if err := copyFixtureFile(previousSource, previous); err != nil {
		t.Fatal(err)
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: upgrade.ChannelPortable, Executable: current}
	if _, err := u.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := fixtureFilesEqual(current, previousSource); err != nil {
		t.Fatalf("first rollback did not activate prior program: %v", err)
	}
	if err := fixtureFilesEqual(previous, currentSource); err != nil {
		t.Fatalf("first rollback did not retain current program: %v", err)
	}
	if _, err := u.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := fixtureFilesEqual(current, currentSource); err != nil {
		t.Fatalf("second rollback did not restore original program: %v", err)
	}
	if err := fixtureFilesEqual(previous, previousSource); err != nil {
		t.Fatalf("second rollback did not restore prior program: %v", err)
	}
}

func TestPortableRollbackGuidesLegacyProgramRecoveryWithCurrentPortableInstaller(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "aigw")
	backup := filepath.Join(filepath.Dir(binary), ".aigw.previous")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("previous-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: upgrade.ChannelPortable, Executable: binary}
	message, err := u.Rollback(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"older program does not support", "current portable package", "installer", "replaces only AIGW", "retains one predecessor"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("rollback recovery guidance missing %q: %s", expected, message)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(binary), "aigw-restore-current")); !os.IsNotExist(err) {
		t.Fatalf("rollback created an extra recovery launcher: %v", err)
	}
}

func TestUpdateKeepsExistingBinaryWhenGitLabAPIChecksumMismatches(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/example-group/example-project/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = w.Write([]byte(strings.Repeat("0", 64) + "  ./" + archiveName + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}, HTTPClient: server.Client()}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Fatalf("old binary replaced after checksum failure: %q", got)
	}
}

func TestUpdateDoesNotForwardGitLabTokenAcrossReleaseRedirect(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	forwardedToken := make(chan string, 1)
	downloadServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedToken <- r.Header.Get("PRIVATE-TOKEN")
		_, _ = w.Write(archive)
	}))
	defer downloadServer.Close()
	gitLabServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/example-group/example-project/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			http.Redirect(w, r, downloadServer.URL+"/asset", http.StatusFound)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  ./%s\n", sum, archiveName)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer gitLabServer.Close()
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", gitLabServer.URL)
	t.Setenv("GITLAB_TOKEN", token)
	transport := gitLabServer.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = downloadServer.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	client := &http.Client{Transport: transport}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}, HTTPClient: client}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if got := <-forwardedToken; got != "" {
		t.Fatalf("GitLab token was forwarded to redirect target: %q", got)
	}
}

func TestUpdateRejectsHTTPSDowngradeRedirectBeforeFollowingIt(t *testing.T) {
	const token = "test-token"
	redirectTargetCalled := make(chan struct{}, 1)
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		redirectTargetCalled <- struct{}{}
	}))
	defer redirectTarget.Close()
	gitLabServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/example-group/example-project/releases/permalink/latest":
			http.Redirect(w, r, redirectTarget.URL+"/latest", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer gitLabServer.Close()
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", gitLabServer.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &missingGlabRunner{},
		HTTPClient: gitLabServer.Client(),
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "HTTPS to HTTP") {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-redirectTargetCalled:
		t.Fatal("client followed HTTPS downgrade redirect")
	default:
	}
}

func TestUpdateRefusesChecksumMismatch(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_linux_amd64/aigw", []byte("new-binary"))
	binary := filepath.Join(t.TempDir(), "aigw")
	_ = os.WriteFile(binary, []byte("old-binary"), 0o755)
	runner := &fakeRunner{
		archive: archive, checksum: strings.Repeat("0", 64) + "  ./aigw_0.2.0_linux_amd64.tar.gz\n",
	}
	u := upgrade.Updater{GOOS: "linux", GOARCH: "amd64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "old-binary" {
		t.Fatalf("old binary replaced after checksum failure: %q", got)
	}
}
