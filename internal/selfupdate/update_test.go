package selfupdate_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/selfupdate"
)

const testReleaseProject = "example-group/example-project"

func TestMain(m *testing.M) {
	_ = os.Setenv("AIGW_RELEASE_HOST", "https://gitlab.example.test")
	_ = os.Setenv("AIGW_RELEASE_PROJECT", testReleaseProject)
	os.Exit(m.Run())
}

type fakeRunner struct {
	archive  []byte
	checksum string
	tag      string
	calls    [][]string
}

type missingGlabRunner struct {
	calls [][]string
}

type emptyDownloadRunner struct {
	calls [][]string
}

type missingDownloadRunner struct {
	calls [][]string
}

type glabAPIAssetRunner struct {
	archive       []byte
	checksum      string
	archiveURL    string
	checksumURL   string
	archiveLabel  string
	checksumLabel string
	calls         [][]string
	fileCalls     [][]string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestUpdateRequiresAnExplicitReleaseSource(t *testing.T) {
	t.Setenv("AIGW_RELEASE_HOST", "")
	t.Setenv("AIGW_RELEASE_PROJECT", "")
	runner := &fakeRunner{}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "release source is not configured") {
		t.Fatalf("Update() error = %v, want explicit release-source configuration error", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("Update() invoked a release client without a release source: %v", runner.calls)
	}
}

func (r *missingGlabRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func (r *emptyDownloadRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		return []byte("v0.2.0\n"), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func (r *missingDownloadRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		return []byte("v0.2.0\n"), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		return nil, os.ErrNotExist
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func (r *glabAPIAssetRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		return []byte("v0.2.0\n"), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		return nil, nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "projects/example-group%2Fexample-project/releases/v0.2.0" {
		archiveLabel := r.archiveLabel
		if archiveLabel == "" {
			archiveLabel = "aigw_0.2.0_darwin_arm64.tar.gz"
		}
		checksumLabel := r.checksumLabel
		if checksumLabel == "" {
			checksumLabel = "checksums.txt"
		}
		return []byte(fmt.Sprintf(`{"assets":{"links":[{"name":%q,"url":%q},{"name":%q,"url":%q}]}}`, archiveLabel, r.archiveURL, checksumLabel, r.checksumURL)), nil
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func (r *glabAPIAssetRunner) RunToFile(_ context.Context, destination, name string, args ...string) error {
	r.fileCalls = append(r.fileCalls, append([]string{name}, args...))
	if name != "glab" || len(args) != 2 || args[0] != "api" {
		return fmt.Errorf("unexpected file command: %s %v", name, args)
	}
	switch args[1] {
	case r.archiveURL:
		return os.WriteFile(destination, r.archive, 0o600)
	case r.checksumURL:
		return os.WriteFile(destination, []byte(r.checksum), 0o600)
	default:
		return fmt.Errorf("unexpected API asset URL: %s", args[1])
	}
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		tag := r.tag
		if tag == "" {
			tag = "v0.2.0"
		}
		if contains(args, "--jq") {
			return []byte(tag + "\n"), nil
		}
		return []byte(`[{"tag_name":"` + tag + `"}]`), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		dir, asset := "", ""
		for i, arg := range args {
			if arg == "--dir" {
				dir = args[i+1]
			}
			if arg == "--asset-name" {
				asset = args[i+1]
			}
		}
		if asset == "checksums.txt" {
			return nil, os.WriteFile(filepath.Join(dir, asset), []byte(r.checksum), 0o600)
		}
		return nil, os.WriteFile(filepath.Join(dir, asset), r.archive, 0o600)
	}
	switch name {
	case "open", "sudo", "msiexec":
		return []byte("ok"), nil
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func TestUpdateInstallsVerifiedLocalCandidateWithoutReleaseSource(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	candidate := t.TempDir()
	if err := os.WriteFile(filepath.Join(candidate, archiveName), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(candidate, "checksums.txt"), []byte(fmt.Sprintf("%x  %s\n", sum, archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_RELEASE_HOST", "")
	t.Setenv("AIGW_RELEASE_PROJECT", "")
	t.Setenv("AIGW_LOCAL_CANDIDATE", candidate)
	runner := &fakeRunner{}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "candidate-binary" {
		t.Fatalf("binary = %q, want candidate-binary", got)
	}
	if !strings.Contains(message, "verified local candidate") || !strings.Contains(message, "v0.2.0") {
		t.Fatalf("message = %q", message)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("local candidate unexpectedly invoked a release client: %v", runner.calls)
	}
}

func TestUpdateRejectsLocalCandidateChecksumMismatchWithoutFallback(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	candidate := t.TempDir()
	if err := os.WriteFile(filepath.Join(candidate, archiveName), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(candidate, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_LOCAL_CANDIDATE", candidate)
	runner := &fakeRunner{}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     runner,
		Release:    selfupdate.ReleaseSource{Host: "https://gitlab.example.test", Project: testReleaseProject},
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary replaced after local candidate checksum failure: %q", got)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("integrity failure must not fall back to a remote release: %v", runner.calls)
	}
}

func TestUpdateDoesNotUseGitHubMirrorAfterGitLabIntegrityFailure(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("bad-primary-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	mirrorRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		mirrorRequests++
		http.Error(w, "mirror must not be contacted", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("AIGW_RELEASE_MIRROR_HOST", server.URL)
	t.Setenv("AIGW_RELEASE_MIRROR_PROJECT", "example-owner/aigw-cli")
	runner := &fakeRunner{archive: archive, checksum: strings.Repeat("0", 64) + "  " + archiveName + "\n"}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     runner,
		Release:    selfupdate.ReleaseSource{Provider: selfupdate.ReleaseProviderGitLab, Host: "https://gitlab.example.test", Project: testReleaseProject},
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	if mirrorRequests != 0 {
		t.Fatalf("GitHub mirror was contacted after a primary integrity failure: %d", mirrorRequests)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary replaced after primary checksum failure: %q", got)
	}
}

func TestUpdateFallsBackToGitHubAfterGitLabAPITransportFailure(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("github-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				archiveName, serverURL+"/downloads/"+archiveName, serverURL+"/downloads/checksums.txt")
		case "/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sum, archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	t.Setenv("AIGW_RELEASE_MIRROR_HOST", server.URL)
	t.Setenv("AIGW_RELEASE_MIRROR_PROJECT", "example-owner/aigw-cli")
	t.Setenv("GITLAB_TOKEN", "test-token")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Host, "gitlab.example.test") {
			return nil, fmt.Errorf("simulated GitLab transport outage")
		}
		return server.Client().Transport.RoundTrip(request)
	})}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     &missingGlabRunner{},
		HTTPClient: client,
		Release:    selfupdate.ReleaseSource{Provider: selfupdate.ReleaseProviderGitLab, Host: "https://gitlab.example.test", Project: testReleaseProject},
	}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "github-binary" || !strings.Contains(message, "github fallback") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
}

func TestUpdateFallsBackToGitHubWhenGitLabIsUnavailable(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("github-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	requests := []string{}
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Path)
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest", "/repos/example-owner/aigw-cli/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				archiveName, serverURL+"/downloads/"+archiveName, serverURL+"/downloads/checksums.txt")
		case "/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sum, archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	githubHost := strings.TrimPrefix(server.URL, "http://")
	t.Setenv("AIGW_RELEASE_HOST", "https://gitlab.example.test")
	t.Setenv("AIGW_RELEASE_PROJECT", testReleaseProject)
	t.Setenv("AIGW_RELEASE_MIRROR_HOST", "http://"+githubHost)
	t.Setenv("AIGW_RELEASE_MIRROR_PROJECT", "example-owner/aigw-cli")
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     &missingGlabRunner{},
		HTTPClient: server.Client(),
	}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "github-binary" || !strings.Contains(message, "github fallback") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
	if len(requests) != 3 || requests[0] != "/repos/example-owner/aigw-cli/releases/latest" {
		t.Fatalf("GitHub fallback requests = %v", requests)
	}
}

func TestUpdateRejectsDuplicateChecksumEntries(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n%x  ./%s\n", sum, name, sum, name)}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
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
		[]byte("unexpected/aigw"), []byte("second"),
	)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n", sum, name)}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "multiple AIGW binaries") {
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
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
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
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
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
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
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
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: selfupdate.ChannelPortable, Executable: binary}
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
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: selfupdate.ChannelPortable, Executable: binary}
	_, err := u.Rollback(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no previous portable AIGW binary") {
		t.Fatalf("error = %v", err)
	}
}

func TestProgramRollbackRejectsPackageManagedInstall(t *testing.T) {
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: selfupdate.ChannelPKG, Executable: filepath.Join(t.TempDir(), "aigw")}
	_, err := u.Rollback(context.Background())
	if err == nil || !strings.Contains(err.Error(), "portable installation") {
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
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: selfupdate.ChannelPortable, Executable: current}
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
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: selfupdate.ChannelPortable, Executable: binary}
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

func TestUpdateUsesSupportedGlabJSONFlags(t *testing.T) {
	runner := &fakeRunner{}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
	}
	if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if len(call) < 3 || call[0] != "glab" || call[1] != "release" || call[2] != "list" {
			continue
		}
		if contains(call, "--format") {
			t.Fatalf("glab release list used retired --format json contract: %v", call)
		}
		if !containsSequence(call, "-F", "json", "--jq", ".[0].tag_name") {
			t.Fatalf("glab release list must use -F json and select the first tag: %v", call)
		}
		return
	}
	t.Fatalf("glab release list was not called: %v", runner.calls)
}

func TestUpdatePassesConfiguredGitLabHostToGlab(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture; Windows command-environment propagation is covered by the native Windows release runner")
	}
	dir := t.TempDir()
	capture := filepath.Join(dir, "gl-host")
	glab := filepath.Join(dir, "glab")
	script := "#!/bin/sh\nprintf '%s' \"$GL_HOST\" > \"$AIGW_TEST_CAPTURE\"\nprintf 'v0.2.0\\n'\n"
	if err := os.WriteFile(glab, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AIGW_RELEASE_HOST", "https://gitlab.example.test")
	t.Setenv("AIGW_TEST_CAPTURE", capture)
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(dir, "aigw"),
		Runner:     selfupdate.ExecRunner{},
	}
	if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
		t.Fatal(err)
	}
	host, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(host); got != "https://gitlab.example.test" {
		t.Fatalf("GL_HOST = %q, want configured self-hosted GitLab URL", got)
	}
}

func TestUpdateFallsBackToGitLabAPIWhenGlabIsUnavailable(t *testing.T) {
	const token = "test-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		if got := r.URL.EscapedPath(); got != "/api/v4/projects/example-group%2Fexample-project/releases/permalink/latest" {
			t.Fatalf("path = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	}))
	defer server.Close()
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &missingGlabRunner{}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
		HTTPClient: server.Client(),
	}
	message, err := u.Update(context.Background(), "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "v0.2.0") {
		t.Fatalf("message = %q", message)
	}
	for _, call := range runner.calls {
		if contains(call, token) {
			t.Fatalf("GitLab token leaked to command invocation: %v", call)
		}
	}
}

func TestUpdateFallsBackWhenExecRunnerCannotFindGlab(t *testing.T) {
	const token = "test-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	}))
	defer server.Close()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     selfupdate.ExecRunner{},
		HTTPClient: server.Client(),
	}
	if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTokenFallbackRequiresExplicitHTTPSGitLabOrigin(t *testing.T) {
	for _, host := range []string{"", "http://gitlab.example.test"} {
		t.Run(host, func(t *testing.T) {
			if host == "" {
				t.Setenv("AIGW_RELEASE_HOST", "")
			} else {
				t.Setenv("AIGW_RELEASE_HOST", host)
			}
			t.Setenv("GITLAB_TOKEN", "test-token")
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("token fallback attempted a request without an explicit HTTPS origin")
				return nil, nil
			})}
			u := selfupdate.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &missingGlabRunner{},
				HTTPClient: client,
			}
			_, err := u.Update(context.Background(), "0.2.0")
			if err == nil || (!strings.Contains(err.Error(), "AIGW_RELEASE_HOST") && !strings.Contains(err.Error(), "release source is incomplete") && !strings.Contains(err.Error(), "release host")) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestUpdateRejectsControlCharacterTokenBeforeGitLabAPIRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("GitLab API was called with an invalid token")
	}))
	defer server.Close()
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", "test-token\ninjected")
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &missingGlabRunner{},
		HTTPClient: server.Client(),
	}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN contains a control character") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateDoesNotExposeTokenInGitLabAPIError(t *testing.T) {
	const token = "do-not-leak-this-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &missingGlabRunner{},
		HTTPClient: server.Client(),
	}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil {
		t.Fatal("expected GitLab API error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("GitLab token leaked to error: %v", err)
	}
}

func TestUpdateDownloadsFromGitLabAPIWhenGlabIsUnavailable(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
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
			_, _ = w.Write([]byte(fmt.Sprintf("%x  ./%s\n", sum, archiveName)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &missingGlabRunner{}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner, HTTPClient: server.Client()}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" || !strings.Contains(message, "v0.2.0") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
	for _, call := range runner.calls {
		if contains(call, token) {
			t.Fatalf("GitLab token leaked to command invocation: %v", call)
		}
	}
}

func TestUpdateFallsBackToGitLabAPIWhenGlabReportsEmptyReleaseDownload(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  ./%s\n", sum, archiveName)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &emptyDownloadRunner{}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner, HTTPClient: server.Client()}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" || !strings.Contains(message, "v0.2.0") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
	if len(runner.calls) < 2 || !containsSequence(runner.calls[1], "glab", "release", "download") {
		t.Fatalf("empty glab download was not attempted: %v", runner.calls)
	}
}

func TestUpdateFallsBackToGitLabAPIWhenGlabReportsMissingDownloadedFile(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  ./%s\n", sum, archiveName)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingDownloadRunner{}, HTTPClient: server.Client()}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("binary=%q", got)
	}
}

func TestUpdateUsesGlabAPIKeychainFallbackWhenReleaseDownloadLeavesNoFile(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	runner := &glabAPIAssetRunner{
		archive:     archive,
		checksum:    fmt.Sprintf("%x  ./%s\n", sum, archiveName),
		archiveURL:  "http://packages.example/aigw/0.2.0/" + archiveName,
		checksumURL: "http://packages.example/aigw/0.2.0/checksums.txt",
	}
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITLAB_TOKEN", "")
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("binary = %q, want new-binary", got)
	}
	if len(runner.fileCalls) != 2 {
		t.Fatalf("glab API asset downloads = %v, want two streamed assets", runner.fileCalls)
	}
	for _, call := range append(runner.calls, runner.fileCalls...) {
		if contains(call, "GITLAB_TOKEN") || contains(call, "test-token") {
			t.Fatalf("credential leaked to glab command: %v", call)
		}
	}
}

func TestUpdateUsesReleaseAssetURLBasenameWhenDisplayNamesDiffer(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	runner := &glabAPIAssetRunner{
		archive:       archive,
		checksum:      fmt.Sprintf("%x  ./%s\n", sum, archiveName),
		archiveURL:    "http://packages.example/aigw/0.2.0/" + archiveName,
		checksumURL:   "http://packages.example/aigw/0.2.0/checksums.txt",
		archiveLabel:  "macOS arm64 portable",
		checksumLabel: "SHA-256 checksums",
	}
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("binary = %q, want new-binary", got)
	}
}

func TestExecRunnerStreamsGlabAPIAssetWithConfiguredGitLabHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	dir := t.TempDir()
	capture := filepath.Join(dir, "gl-hosts")
	glab := filepath.Join(dir, "glab")
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	archiveSource := filepath.Join(dir, "archive")
	checksumSource := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(archiveSource, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checksumSource, []byte(fmt.Sprintf("%x  ./%s\n", sum, archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
printf '%s\n' "$GL_HOST" >> "$AIGW_TEST_CAPTURE"
case "$1:$2" in
  release:list) printf 'v0.2.0\n' ;;
  release:download) exit 0 ;;
  api:projects/example-group%2Fexample-project/releases/v0.2.0) printf '{"assets":{"links":[{"name":"aigw_0.2.0_darwin_arm64.tar.gz","url":"http://packages.example/aigw_0.2.0_darwin_arm64.tar.gz"},{"name":"checksums.txt","url":"http://packages.example/checksums.txt"}]}}' ;;
  api:http://packages.example/aigw_0.2.0_darwin_arm64.tar.gz) cat "$AIGW_TEST_ARCHIVE" ;;
  api:http://packages.example/checksums.txt) cat "$AIGW_TEST_CHECKSUMS" ;;
  *) echo "unexpected glab arguments: $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(glab, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AIGW_TEST_CAPTURE", capture)
	t.Setenv("AIGW_TEST_ARCHIVE", archiveSource)
	t.Setenv("AIGW_TEST_CHECKSUMS", checksumSource)
	t.Setenv("AIGW_RELEASE_HOST", "https://gitlab.example.test")
	binary := filepath.Join(dir, "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: selfupdate.ExecRunner{}}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("binary = %q", got)
	}
	hosts, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range strings.Fields(string(hosts)) {
		if host != "https://gitlab.example.test" {
			t.Fatalf("GL_HOST = %q, want configured GitLab host for every glab call", host)
		}
	}
}

func TestExecRunnerBoundsStderrWithoutBreakingCommandOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "noisy")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nhead -c 32768 /dev/zero | tr '\\0' x >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := (selfupdate.ExecRunner{}).RunToFile(context.Background(), filepath.Join(dir, "asset"), script)
	if err == nil {
		t.Fatal("RunToFile succeeded for failing command")
	}
	if len(err.Error()) > 17<<10 {
		t.Fatalf("unbounded command error length = %d", len(err.Error()))
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
	t.Setenv("AIGW_RELEASE_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}, HTTPClient: server.Client()}
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
	t.Setenv("AIGW_RELEASE_HOST", gitLabServer.URL)
	t.Setenv("GITLAB_TOKEN", token)
	transport := gitLabServer.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = downloadServer.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	client := &http.Client{Transport: transport}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}, HTTPClient: client}
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
	t.Setenv("AIGW_RELEASE_HOST", gitLabServer.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{
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

func TestUpdateRefusesOlderStableReleaseWithoutReplacingBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.1.9_darwin_arm64/aigw", []byte("old-release-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.1.9_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.1.9"}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "older") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "current-binary" {
		t.Fatalf("binary was replaced by older release: %q", got)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("older release should fail before downloading assets: %v", runner.calls)
	}
}

func TestUpdateRefusesOlderPrereleaseWithoutReplacingBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0-rc.1_darwin_arm64/aigw", []byte("old-release-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0-rc.1_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.2.0-rc.1"}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.2.0-rc.2")
	if err == nil || !strings.Contains(err.Error(), "older") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "current-binary" {
		t.Fatalf("binary was replaced by older prerelease: %q", got)
	}
}

func TestUpdateAcceptsStableReleaseAfterPrerelease(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("stable-release-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("prerelease-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.2.0"}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.2.0-rc.2"); err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "stable-release-binary" {
		t.Fatalf("binary = %q", got)
	}
}

func TestUpdateAcceptsNumericallyNewerPrerelease(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0-rc.10_darwin_arm64/aigw", []byte("newer-prerelease-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0-rc.10_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("older-prerelease-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.2.0-rc.10"}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.2.0-rc.9"); err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "newer-prerelease-binary" {
		t.Fatalf("binary = %q", got)
	}
}

func TestUpdateRejectsInvalidCurrentVersionBeforeDownloading(t *testing.T) {
	runner := &fakeRunner{tag: "v0.2.0"}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
	}
	_, err := u.Update(context.Background(), "development-build")
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("error = %v", err)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("invalid current version should fail before downloading assets: %v", runner.calls)
	}
}

func TestUpdateRejectsMalformedReleaseVersionBeforeDownloading(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{tag: "release-candidate"}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("error = %v", err)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("malformed release tag should fail before downloading assets: %v", runner.calls)
	}
}

func TestUpdateRejectsOverflowingReleaseVersionBeforeDownloading(t *testing.T) {
	runner := &fakeRunner{tag: "v18446744073709551616.0.0"}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
	}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("error = %v", err)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("overflowing release tag should fail before downloading assets: %v", runner.calls)
	}
}

func TestUpdateRejectsGitLabHostWithCredentialsPathQueryOrFragment(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "test-token")
	for _, host := range []string{
		"https://user:password@gitlab.example.test",
		"https://gitlab.example.test/prefix",
		"https://gitlab.example.test?token=leak",
		"https://gitlab.example.test#fragment",
	} {
		t.Run(host, func(t *testing.T) {
			t.Setenv("AIGW_RELEASE_HOST", host)
			u := selfupdate.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &missingGlabRunner{},
			}
			_, err := u.Update(context.Background(), "0.2.0")
			if err == nil || !strings.Contains(err.Error(), "release host") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestUpdateGitLabFallbackUsesBoundedHTTPClientTimeout(t *testing.T) {
	for name, client := range map[string]*http.Client{
		"default":         {},
		"caller deadline": {Timeout: 37 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			deadline := make(chan time.Duration, 1)
			client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				when, ok := request.Context().Deadline()
				if !ok {
					t.Fatal("GitLab fallback request has no deadline")
				}
				deadline <- time.Until(when)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.2.0"}`)),
					Header:     make(http.Header),
					Request:    request,
				}, nil
			})
			t.Setenv("AIGW_RELEASE_HOST", "https://gitlab.example.test")
			t.Setenv("GITLAB_TOKEN", "test-token")
			u := selfupdate.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &missingGlabRunner{},
				HTTPClient: client,
			}
			if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
				t.Fatal(err)
			}
			remaining := <-deadline
			if remaining < 25*time.Second || remaining > 40*time.Second {
				t.Fatalf("request deadline remaining = %s, want a bounded 30s default or preserved 37s caller timeout", remaining)
			}
		})
	}
}

func TestUpdateRefusesChecksumMismatch(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_linux_amd64/aigw", []byte("new-binary"))
	binary := filepath.Join(t.TempDir(), "aigw")
	_ = os.WriteFile(binary, []byte("old-binary"), 0o755)
	runner := &fakeRunner{
		archive: archive, checksum: strings.Repeat("0", 64) + "  ./aigw_0.2.0_linux_amd64.tar.gz\n",
	}
	u := selfupdate.Updater{GOOS: "linux", GOARCH: "amd64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "old-binary" {
		t.Fatalf("old binary replaced after checksum failure: %q", got)
	}
}

func TestWindowsReplacementPlanRetainsImmediatePreviousBinary(t *testing.T) {
	executable := `C:\\Users\\test\\aigw.exe`
	plan, err := selfupdate.WindowsReplacementPlan(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`if exist "C:\\Users\\test\\aigw.exe" move /Y "C:\\Users\\test\\aigw.exe" "C:\\Users\\test\\.aigw.previous.exe"`,
		`move /Y "C:\\Users\\test\\aigw.exe.update" "C:\\Users\\test\\aigw.exe"`,
	} {
		if !strings.Contains(plan, expected) {
			t.Fatalf("Windows replacement plan missing %q:\n%s", expected, plan)
		}
	}
}

func TestWindowsReplacementPlanRejectsEmptyExecutable(t *testing.T) {
	if _, err := selfupdate.WindowsReplacementPlan(" "); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestWindowsReplacementPlanUsesPortableRollbackNameForForwardSlashPath(t *testing.T) {
	plan, err := selfupdate.WindowsReplacementPlan("C:/Users/test/aigw.exe")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, `"C:/Users/test/.aigw.previous.exe"`) {
		t.Fatalf("Windows replacement plan does not use portable rollback name:\n%s", plan)
	}
}

func TestWindowsRollbackPlanStagesPriorBinaryAndRestoresTheOriginalPairOnFailure(t *testing.T) {
	executable := `C:\\Users\\test\\aigw.exe`
	plan, err := selfupdate.WindowsRollbackPlan(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`move /Y "C:\\Users\\test\\aigw.exe" "C:\\Users\\test\\.aigw.previous.exe"`,
		`move /Y "C:\\Users\\test\\aigw.exe.rollback" "C:\\Users\\test\\aigw.exe"`,
		`move /Y "C:\\Users\\test\\.aigw.previous.exe" "C:\\Users\\test\\aigw.exe"`,
		`move /Y "C:\\Users\\test\\aigw.exe.rollback" "C:\\Users\\test\\.aigw.previous.exe"`,
		`del "C:\\Users\\test\\aigw.exe.rollback" > nul 2>&1`,
		`ping 127.0.0.1 -n 3 > nul`,
		`if errorlevel 1 goto :failed_before_swap`,
		`if not errorlevel 1 goto :success`,
	} {
		if !strings.Contains(plan, expected) {
			t.Fatalf("Windows rollback plan missing %q:\n%s", expected, plan)
		}
	}
	if strings.Contains(plan, "http://") || strings.Contains(plan, "https://") {
		t.Fatalf("Windows rollback plan must not contain a network endpoint:\n%s", plan)
	}
	if strings.Contains(plan, `if exist "C:\\Users\\test\\aigw.exe"`) {
		t.Fatalf("Windows rollback plan must not treat an unchanged current binary as success:\n%s", plan)
	}
}

func TestWindowsRollbackPlanRejectsEmptyExecutable(t *testing.T) {
	if _, err := selfupdate.WindowsRollbackPlan(" \t"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestPackageManagedDebUpdateDownloadsVerifiesAndInvokesPackageManager(t *testing.T) {
	payload := []byte("deb package")
	sum := sha256.Sum256(payload)
	name := "aigw_0.2.0_linux_amd64.deb"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: payload, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := selfupdate.Updater{GOOS: "linux", GOARCH: "amd64", Channel: selfupdate.ChannelDeb, Executable: binary, Runner: runner}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "old-binary" {
		t.Fatalf("package-managed update replaced binary directly: %q", got)
	}
	if !strings.Contains(message, "package manager") {
		t.Fatalf("message = %q", message)
	}
	if !runner.called("sudo", "dpkg", "-i") {
		t.Fatalf("dpkg install was not invoked; calls=%v", runner.calls)
	}
}

func TestMacPackageUpdateUsesUniversalPkgAsset(t *testing.T) {
	payload := []byte("pkg package")
	sum := sha256.Sum256(payload)
	name := "aigw_0.2.0_darwin_universal.pkg"
	runner := &fakeRunner{archive: payload, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: selfupdate.ChannelPKG, Executable: filepath.Join(t.TempDir(), "aigw"), Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if !runner.downloaded(name) {
		t.Fatalf("universal pkg asset was not downloaded; calls=%v", runner.calls)
	}
	if !runner.called("open") {
		t.Fatalf("macOS installer was not opened; calls=%v", runner.calls)
	}
}

func TestCurrentUsesBuildTimeInstallChannel(t *testing.T) {
	previous := selfupdate.InstallChannel
	t.Cleanup(func() { selfupdate.InstallChannel = previous })
	selfupdate.InstallChannel = "rpm"
	updater := selfupdate.Current("/usr/bin/aigw")
	if updater.Channel != selfupdate.ChannelRPM {
		t.Fatalf("channel = %q, want rpm", updater.Channel)
	}
}

func TestCurrentUsesBuildTimeReleaseProvider(t *testing.T) {
	previousProvider, previousHost, previousProject := selfupdate.BuildReleaseProvider, selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject
	t.Cleanup(func() {
		selfupdate.BuildReleaseProvider, selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject = previousProvider, previousHost, previousProject
	})
	selfupdate.BuildReleaseProvider = "github"
	selfupdate.BuildReleaseHost = "https://github.com"
	selfupdate.BuildReleaseProject = "HengYangDS/aigw-cli"
	t.Setenv("AIGW_RELEASE_PROVIDER", "")
	t.Setenv("AIGW_RELEASE_HOST", "")
	t.Setenv("AIGW_RELEASE_PROJECT", "")
	updater := selfupdate.Current(filepath.Join(t.TempDir(), "aigw"))
	if got := updater.Release; got != (selfupdate.ReleaseSource{Provider: selfupdate.ReleaseProviderGitHub, Host: "https://github.com", Project: "HengYangDS/aigw-cli"}) {
		t.Fatalf("primary source = %#v", got)
	}
}

func TestCurrentUsesBuildTimeGitHubPrimarySource(t *testing.T) {
	previousHost, previousProject := selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject
	t.Cleanup(func() {
		selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject = previousHost, previousProject
	})
	selfupdate.BuildReleaseHost = "https://github.com"
	selfupdate.BuildReleaseProject = "HengYangDS/aigw-cli"
	t.Setenv("AIGW_RELEASE_HOST", "")
	t.Setenv("AIGW_RELEASE_PROJECT", "")
	updater := selfupdate.Current(filepath.Join(t.TempDir(), "aigw"))
	if got := updater.Release; got != (selfupdate.ReleaseSource{Provider: selfupdate.ReleaseProviderGitLab, Host: "https://github.com", Project: "HengYangDS/aigw-cli"}) {
		t.Fatalf("primary source = %#v", got)
	}
}

func TestCurrentUsesBuildTimeGitHubMirrorSource(t *testing.T) {
	previousHost, previousProject := selfupdate.BuildReleaseMirrorHost, selfupdate.BuildReleaseMirrorProject
	t.Cleanup(func() {
		selfupdate.BuildReleaseMirrorHost, selfupdate.BuildReleaseMirrorProject = previousHost, previousProject
	})
	selfupdate.BuildReleaseMirrorHost = "https://github.com"
	selfupdate.BuildReleaseMirrorProject = "example-owner/aigw-cli"
	t.Setenv("AIGW_RELEASE_MIRROR_HOST", "")
	t.Setenv("AIGW_RELEASE_MIRROR_PROJECT", "")
	updater := selfupdate.Current(filepath.Join(t.TempDir(), "aigw"))
	if got := updater.Mirror; got != (selfupdate.ReleaseSource{Provider: selfupdate.ReleaseProviderGitHub, Host: "https://github.com", Project: "example-owner/aigw-cli"}) {
		t.Fatalf("mirror source = %#v", got)
	}
}

func TestCurrentUsesBuildTimeReleaseSource(t *testing.T) {
	previousHost, previousProject := selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject
	t.Cleanup(func() {
		selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject = previousHost, previousProject
	})
	selfupdate.BuildReleaseHost = "https://gitlab.example.test"
	selfupdate.BuildReleaseProject = testReleaseProject
	t.Setenv("AIGW_RELEASE_HOST", "")
	t.Setenv("AIGW_RELEASE_PROJECT", "")
	updater := selfupdate.Current(filepath.Join(t.TempDir(), "aigw"))
	if got := updater.Release; got != (selfupdate.ReleaseSource{Provider: selfupdate.ReleaseProviderGitLab, Host: "https://gitlab.example.test", Project: testReleaseProject}) {
		t.Fatalf("release source = %#v", got)
	}
}

func TestExplicitReleaseSourceEnvironmentOverridesBuildMetadata(t *testing.T) {
	previousHost, previousProject := selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject
	t.Cleanup(func() {
		selfupdate.BuildReleaseHost, selfupdate.BuildReleaseProject = previousHost, previousProject
	})
	selfupdate.BuildReleaseHost = "https://embedded.example.test"
	selfupdate.BuildReleaseProject = "embedded/project"
	t.Setenv("AIGW_RELEASE_HOST", "https://override.example.test")
	t.Setenv("AIGW_RELEASE_PROJECT", testReleaseProject)
	runner := &fakeRunner{}
	u := selfupdate.Current(filepath.Join(t.TempDir(), "aigw"))
	u.Runner = runner
	if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if containsSequence(call, "-R", testReleaseProject) {
			return
		}
	}
	t.Fatalf("release project override was not passed to glab: %v", runner.calls)
}

func (r *fakeRunner) downloaded(asset string) bool {
	for _, call := range r.calls {
		for i, part := range call {
			if part == "--asset-name" && i+1 < len(call) && call[i+1] == asset {
				return true
			}
		}
	}
	return false
}

func (r *fakeRunner) called(prefix ...string) bool {
	for _, call := range r.calls {
		if len(call) < len(prefix) {
			continue
		}
		ok := true
		for i := range prefix {
			if call[i] != prefix[i] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsSequence(values []string, want ...string) bool {
	for i := 0; i+len(want) <= len(values); i++ {
		match := true
		for j := range want {
			if values[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func copyFixtureFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o755)
}

func fixtureFilesEqual(left, right string) error {
	leftData, err := os.ReadFile(left)
	if err != nil {
		return err
	}
	rightData, err := os.ReadFile(right)
	if err != nil {
		return err
	}
	if !bytes.Equal(leftData, rightData) {
		return fmt.Errorf("contents differ")
	}
	return nil
}

func tarGzEntries(t *testing.T, entries ...[]byte) []byte {
	t.Helper()
	if len(entries)%2 != 0 {
		t.Fatal("tar fixture needs name/data pairs")
	}
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for index := 0; index < len(entries); index += 2 {
		name, data := string(entries[index]), entries[index+1]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func tarGz(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
