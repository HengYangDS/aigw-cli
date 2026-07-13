package selfupdate_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/selfupdate"
)

type fakeRunner struct {
	archive  []byte
	checksum string
	calls    [][]string
}

type missingGlabRunner struct {
	calls [][]string
}

func (r *missingGlabRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		if contains(args, "--jq") {
			return []byte("v0.2.0\n"), nil
		}
		return []byte(`[{"tag_name":"v0.2.0"}]`), nil
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
	dir := t.TempDir()
	capture := filepath.Join(dir, "gl-host")
	glab := filepath.Join(dir, "glab")
	script := "#!/bin/sh\nprintf '%s' \"$GL_HOST\" > \"$AIGW_TEST_CAPTURE\"\nprintf 'v0.2.0\\n'\n"
	if err := os.WriteFile(glab, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AIGW_GL_HOST", "https://gitlab.example.test")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		if got := r.URL.EscapedPath(); got != "/api/v4/projects/dig%2Fmisc%2Fagentic-third-party-api%2Faigw-cli/releases/permalink/latest" {
			t.Fatalf("path = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	}))
	defer server.Close()
	t.Setenv("AIGW_GL_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &missingGlabRunner{}
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	}))
	defer server.Close()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("AIGW_GL_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     selfupdate.ExecRunner{},
	}
	if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRejectsControlCharacterTokenBeforeGitLabAPIRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("GitLab API was called with an invalid token")
	}))
	defer server.Close()
	t.Setenv("AIGW_GL_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", "test-token\ninjected")
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &missingGlabRunner{},
	}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN contains a control character") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateDoesNotExposeTokenInGitLabAPIError(t *testing.T) {
	const token = "do-not-leak-this-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	t.Setenv("AIGW_GL_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &missingGlabRunner{},
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/dig/misc/agentic-third-party-api/aigw-cli/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/dig/misc/agentic-third-party-api/aigw-cli/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/dig/misc/agentic-third-party-api/aigw-cli/-/releases/v0.2.0/downloads/checksums.txt":
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
	t.Setenv("AIGW_GL_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &missingGlabRunner{}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
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

func TestUpdateKeepsExistingBinaryWhenGitLabAPIChecksumMismatches(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/dig/misc/agentic-third-party-api/aigw-cli/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/dig/misc/agentic-third-party-api/aigw-cli/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/dig/misc/agentic-third-party-api/aigw-cli/-/releases/v0.2.0/downloads/checksums.txt":
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
	t.Setenv("AIGW_GL_HOST", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}}
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
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedToken <- r.Header.Get("PRIVATE-TOKEN")
		_, _ = w.Write(archive)
	}))
	defer downloadServer.Close()
	gitLabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/dig/misc/agentic-third-party-api/aigw-cli/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/dig/misc/agentic-third-party-api/aigw-cli/-/releases/v0.2.0/downloads/" + archiveName:
			http.Redirect(w, r, downloadServer.URL+"/asset", http.StatusFound)
		case "/dig/misc/agentic-third-party-api/aigw-cli/-/releases/v0.2.0/downloads/checksums.txt":
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
	t.Setenv("AIGW_GL_HOST", gitLabServer.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if got := <-forwardedToken; got != "" {
		t.Fatalf("GitLab token was forwarded to redirect target: %q", got)
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
	if !strings.Contains(message, "包管理器") {
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
