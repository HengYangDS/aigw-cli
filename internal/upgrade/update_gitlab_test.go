package upgrade_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/upgrade"
)

func TestUpdateUsesSupportedGlabJSONFlags(t *testing.T) {
	runner := &fakeRunner{}
	u := upgrade.Updater{
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
	t.Setenv("AIGW_TEST_CAPTURE", capture)
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(dir, "aigw"),
		Runner:     upgrade.ExecRunner{},
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &missingGlabRunner{}
	u := upgrade.Updater{
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     upgrade.ExecRunner{},
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
				t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
			} else {
				t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", host)
			}
			t.Setenv("GITLAB_TOKEN", "test-token")
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("token fallback attempted a request without an explicit HTTPS origin")
				return nil, nil
			})}
			u := upgrade.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &missingGlabRunner{},
				HTTPClient: client,
			}
			_, err := u.Update(context.Background(), "0.2.0")
			if err == nil || (!strings.Contains(err.Error(), "AIGW_GITLAB_RELEASE_ORIGIN") && !strings.Contains(err.Error(), "release source is incomplete") && !strings.Contains(err.Error(), "release origin")) {
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", "test-token\ninjected")
	u := upgrade.Updater{
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &missingGlabRunner{}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner, HTTPClient: server.Client()}
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	runner := &emptyDownloadRunner{}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner, HTTPClient: server.Client()}
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingDownloadRunner{}, HTTPClient: server.Client()}
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
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
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
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
	binary := filepath.Join(dir, "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: upgrade.ExecRunner{}}
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
	err := (upgrade.ExecRunner{}).RunToFile(context.Background(), filepath.Join(dir, "asset"), script)
	if err == nil {
		t.Fatal("RunToFile succeeded for failing command")
	}
	if len(err.Error()) > 17<<10 {
		t.Fatalf("unbounded command error length = %d", len(err.Error()))
	}
}

func TestExecRunnerKeepsSuccessfulStdoutIndependentOfStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "warning")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'release-tag\\n'\nprintf 'configuration warning\\n' >&2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	output, err := (upgrade.ExecRunner{}).Run(context.Background(), script)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(output), "release-tag\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
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
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", host)
			u := upgrade.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &missingGlabRunner{},
			}
			_, err := u.Update(context.Background(), "0.2.0")
			if err == nil || !strings.Contains(err.Error(), "release origin") {
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
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
			t.Setenv("GITLAB_TOKEN", "test-token")
			u := upgrade.Updater{
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
