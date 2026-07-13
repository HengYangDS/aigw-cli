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
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/selfupdate"
)

type fakeRunner struct {
	archive  []byte
	checksum string
	tag      string
	calls    [][]string
}

type missingGlabRunner struct {
	calls [][]string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func (r *missingGlabRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
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
	if info.Mode().Perm() != 0o755 {
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
	if info.Mode().Perm() != 0o755 {
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
	if !strings.Contains(message, "已恢复上一程序版本") {
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
	for _, expected := range []string{"旧版本不支持", "团队发布页", "当前便携包", "安装脚本", "仅替换 AIGW 程序"} {
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
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	t.Setenv("AIGW_GL_HOST", server.URL)
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
				t.Setenv("AIGW_GL_HOST", "")
			} else {
				t.Setenv("AIGW_GL_HOST", host)
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
			if err == nil || !strings.Contains(err.Error(), "AIGW_GL_HOST") {
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
	t.Setenv("AIGW_GL_HOST", server.URL)
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
	t.Setenv("AIGW_GL_HOST", server.URL)
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

func TestUpdateKeepsExistingBinaryWhenGitLabAPIChecksumMismatches(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		case "/api/v4/projects/dig/misc/agentic-third-party-api/aigw-cli/releases/permalink/latest":
			http.Redirect(w, r, redirectTarget.URL+"/latest", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer gitLabServer.Close()
	t.Setenv("AIGW_GL_HOST", gitLabServer.URL)
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
			t.Setenv("AIGW_GL_HOST", host)
			u := selfupdate.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &missingGlabRunner{},
			}
			_, err := u.Update(context.Background(), "0.2.0")
			if err == nil || !strings.Contains(err.Error(), "AIGW_GL_HOST") {
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
			t.Setenv("AIGW_GL_HOST", "https://gitlab.example.test")
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
