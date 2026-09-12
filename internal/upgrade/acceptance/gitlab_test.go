package acceptance_test

import (
	"aigw-cli/internal/process"
	"aigw-cli/internal/upgrade"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

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

func (r *glabAPIAssetRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	name, args := plan.Executable, plan.Args
	r.calls = append(r.calls, append([]string{name}, args...))
	if filepath.IsAbs(name) && slices.Equal(args, []string{"--version"}) {
		return []byte("aigw version 0.2.0\n"), nil
	}
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

func (r *glabAPIAssetRunner) RunToFile(_ context.Context, destination string, plan process.Plan) error {
	name, args := plan.Executable, plan.Args
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

func TestUpdateUsesSupportedGlabJSONFlags(t *testing.T) {
	runner := &releaseRunner{}
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
		if slices.Contains(call, "--format") {
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
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.EscapedPath() != "/api/v4/projects/example-group%2Fexample-project/releases" || request.Header.Get("Private-Token") != "synthetic-loopback-only" {
			t.Errorf("release request did not bind its source: %s", request.URL)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `[{"tag_name":"v0.2.0"}]`)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("no_prompt: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"AIGW_GITLAB_RELEASE_ORIGIN": server.URL,
		"GLAB_CONFIG_DIR":            dir, "GITLAB_TOKEN": "synthetic-loopback-only", "USE_KEYRING": "false",
		"GLAB_NO_PROMPT": "true", "GLAB_CHECK_UPDATE": "false", "GLAB_SHOW_WHATS_NEW": "false", "GLAB_SEND_TELEMETRY": "false",
	} {
		t.Setenv(name, value)
	}
	u := upgrade.Updater{
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Executable: filepath.Join(dir, "aigw"),
		Runner:     process.Runner{},
	}
	if result, err := u.Update(t.Context(), "0.2.0"); err != nil || result != "already running the latest version v0.2.0" {
		t.Fatalf("release lookup = %q, %v", result, err)
	}
	if requests.Load() != 1 {
		t.Fatalf("selected source requests = %d", requests.Load())
	}
}

func TestUpdateFallsBackToGitLabAPIWhenGlabIsUnavailable(t *testing.T) {
	const token = "test-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Private-Token"); got != token {
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
	runner := &unavailableRunner{}
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
		if slices.Contains(call, token) {
			t.Fatalf("GitLab token leaked to command invocation: %v", call)
		}
	}
}

func TestUpdateFallsBackWhenGlabIsAbsent(t *testing.T) {
	const token = "test-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Private-Token"); got != token {
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
		Runner:     process.Runner{},
		HTTPClient: server.Client(),
	}
	if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateDownloadsFromGitLabAPIWhenGlabIsUnavailable(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Private-Token"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/example-group/example-project/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = fmt.Fprintf(w, "%x  ./%s\n", sum, archiveName)
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
	runner := &unavailableRunner{}
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
		if slices.Contains(call, token) {
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
		if got := r.Header.Get("Private-Token"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = fmt.Fprintf(w, "%x  ./%s\n", sum, archiveName)
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
	runner := &releaseRunner{download: func(string, string) error { return nil }}
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
		if got := r.Header.Get("Private-Token"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = fmt.Fprintf(w, "%x  ./%s\n", sum, archiveName)
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
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &releaseRunner{download: func(string, string) error { return os.ErrNotExist }}, HTTPClient: server.Client()}
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
		if slices.Contains(call, "GITLAB_TOKEN") || slices.Contains(call, "test-token") {
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

func TestUpdateStreamsGlabAssetsWithConfiguredHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	dir := t.TempDir()
	capture := filepath.Join(dir, "gl-hosts")
	glab := filepath.Join(dir, "glab")
	program := "#!/bin/sh\nprintf 'aigw version 0.2.0\\n'\n"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte(program))
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
printf '%s\n' "$GITLAB_HOST" >> "$AIGW_TEST_CAPTURE"
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
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: process.Runner{}}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != program {
		t.Fatalf("binary = %q", got)
	}
	hosts, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	for host := range strings.FieldsSeq(string(hosts)) {
		if host != "https://gitlab.example.test" {
			t.Fatalf("GITLAB_HOST = %q, want configured GitLab host for every glab call", host)
		}
	}
}

func TestUpdateIgnoresGlabConfigurationWarningAroundLatestTag(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitlab := &releaseRunner{archive: archive, checksum: checksum, tag: "Warning: Multiple config files found.\nUsing: /tmp/glab/configuration.yml\nv0.2.0\n"}
	var githubURL string
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest", "/repos/example-owner/aigw-cli/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				archiveName, githubURL+"/downloads/"+archiveName, githubURL+"/downloads/checksums.txt")
		case "/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/downloads/checksums.txt":
			_, _ = w.Write([]byte(checksum))
		default:
			http.NotFound(w, request)
		}
	}))
	defer github.Close()
	githubURL = github.URL
	u := upgrade.Updater{
		GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: gitlab, HTTPClient: github.Client(),
		GitLab: upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
		GitHub: upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitHub, Origin: github.URL, Repository: "example-owner/aigw-cli"},
	}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "gitlab and github") {
		t.Fatalf("message = %q", message)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "candidate-binary" {
		t.Fatalf("binary=%q error=%v", got, err)
	}
}
