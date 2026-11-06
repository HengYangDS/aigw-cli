package program

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// releaseCall is one request that the updater sends to a release source, with
// the headers that it sends and the words of its failures. status is the whole
// failure for a 418 answer, transfer the start of the failure for a refused
// connection, before the transport's own words, and parse the whole failure
// for a body that is not JSON; a download has no parse failure. limit is the
// number of body bytes that a metadata request reads.
type releaseCall struct {
	name                                string
	run                                 func(t *testing.T, u Updater, origin, directory string) error
	accept, authorization, privateToken string
	status, transfer, parse             string
	limit                               int
	// document returns a metadata body of exactly size bytes that the call
	// accepts.
	document func(size int) string
}

func releaseCalls() []releaseCall {
	github := func(origin string) ReleaseSource {
		return ReleaseSource{Provider: ReleaseProviderGitHub, Origin: origin, Repository: "o/r"}
	}
	gitlab := func(origin string) ReleaseSource {
		return ReleaseSource{Provider: ReleaseProviderGitLab, Origin: origin, Repository: "group/project"}
	}
	release := func(size int) string { return padded(`{"tag_name":"v1.0.0","padding":"`, `"}`, size) }
	return []releaseCall{
		{
			name: "public GitHub release", accept: "application/json", limit: 4 << 20, document: release,
			run: func(t *testing.T, u Updater, origin, _ string) error {
				_, err := u.publicLatestTag(t.Context(), github(origin))
				return err
			},
			status: "github query public release metadata: 418 I'm a teapot", transfer: "query public release metadata: Get ",
			parse: "parse public release metadata: invalid character '<' looking for beginning of value",
		},
		{
			name: "public GitLab release", accept: "application/json", limit: 4 << 20, document: release,
			run: func(t *testing.T, u Updater, origin, _ string) error {
				_, err := u.publicLatestTag(t.Context(), gitlab(origin))
				return err
			},
			status: "gitlab query public release metadata: 418 I'm a teapot", transfer: "query public release metadata: Get ",
			parse: "parse public release metadata: invalid character '<' looking for beginning of value",
		},
		{
			name: "GitHub release", accept: "application/vnd.github+json", authorization: "Bearer github-fixture", limit: 4 << 20, document: release,
			run: func(t *testing.T, u Updater, origin, _ string) error {
				_, err := u.githubRelease(t.Context(), github(origin), "releases/latest")
				return err
			},
			status: "github query release metadata: 418 I'm a teapot", transfer: "query GitHub release metadata: Get ",
			parse: "parse GitHub release metadata: invalid character '<' looking for beginning of value",
		},
		{
			name: "GitHub prerelease", accept: "application/vnd.github+json", authorization: "Bearer github-fixture", limit: 4 << 20,
			document: func(size int) string {
				return padded(`[{"tag_name":"v1.0.0-rc.1","prerelease":true,"published_at":"2026-01-01T00:00:00Z","padding":"`, `"}]`, size)
			},
			run: func(t *testing.T, u Updater, origin, _ string) error {
				_, err := u.latestPrereleaseTagFromGitHub(t.Context(), github(origin), githubNotFoundError())
				return err
			},
			// Only a rate limit or a server failure replaces the original 404.
			status: githubNotFoundError().Error(), transfer: "query GitHub prerelease metadata: Get ",
			parse: "parse GitHub prerelease metadata: invalid character '<' looking for beginning of value",
		},
		{
			name: "GitLab release", privateToken: "gitlab-fixture", limit: 1 << 20, document: release,
			run: func(t *testing.T, u Updater, origin, _ string) error {
				u.GitLab = gitlab(origin)
				_, err := u.latestTagFromGitLabAPI(t.Context())
				return err
			},
			status: "gitlab query latest release: 418 I'm a teapot", transfer: "query GitLab latest release: Get ",
			parse: "parse GitLab latest release: invalid character '<' looking for beginning of value",
		},
		{
			name: "GitHub asset", authorization: "Bearer github-fixture",
			run: func(t *testing.T, u Updater, origin, directory string) error {
				return downloadListedAsset(t, u, origin+"/asset.tar.gz", filepath.Join(directory, "asset.tar.gz"))
			},
			status:   "download GitHub release asset asset.tar.gz: github download release asset: 418 I'm a teapot",
			transfer: "download GitHub release asset asset.tar.gz: Get ",
		},
		{
			name: "GitLab asset", privateToken: "gitlab-fixture",
			run: func(t *testing.T, u Updater, origin, directory string) error {
				u.GitLab = gitlab(origin)
				return u.downloadReleaseAssetFromGitLabAPI(t.Context(), "v1.0.0", "asset.tar.gz", directory)
			},
			status:   "gitlab download release asset asset.tar.gz: 418 I'm a teapot",
			transfer: "download release asset asset.tar.gz: Get ",
		},
	}
}

// downloadListedAsset downloads the asset named by destination's base name,
// which release metadata lists at assetURL, into destination's directory, as
// an update does.
func downloadListedAsset(t *testing.T, u Updater, assetURL, destination string) error {
	t.Helper()
	listed := githubRelease{}
	listed.Assets = append(listed.Assets, struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{Name: filepath.Base(destination), BrowserDownloadURL: assetURL})
	return u.downloadGitHubReleaseAssets(t.Context(), listed, filepath.Dir(destination), filepath.Base(destination))
}

// padded returns prefix and suffix around enough padding to make size bytes.
func padded(prefix, suffix string, size int) string {
	return prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix
}

// releaseServer answers every request with status and body, and records the
// headers of the last request.
type releaseServer struct {
	*httptest.Server
	mutex  sync.Mutex
	status int
	body   string
	header http.Header
}

func newReleaseServer(t *testing.T) *releaseServer {
	t.Helper()
	server := &releaseServer{status: http.StatusOK}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.mutex.Lock()
		defer server.mutex.Unlock()
		server.header = r.Header.Clone()
		w.WriteHeader(server.status)
		_, _ = w.Write([]byte(server.body))
	}))
	t.Cleanup(server.Close)
	return server
}

func (server *releaseServer) answer(status int, body string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.status, server.body, server.header = status, body, nil
}

// Every release request sends its own headers, sends no credential when it is
// public, and keeps the words of each of its failures.
func TestReleaseRequestsKeepTheirHeadersAndFailures(t *testing.T) {
	t.Setenv("AIGW_GITHUB_TOKEN", "github-fixture")
	t.Setenv("GITLAB_TOKEN", "gitlab-fixture")
	for _, call := range releaseCalls() {
		t.Run(call.name, func(t *testing.T) {
			server := newReleaseServer(t)
			u := Updater{HTTPClient: server.Client()}
			body := "asset bytes"
			if call.document != nil {
				body = call.document(256)
			}
			server.answer(http.StatusOK, body)
			directory := t.TempDir()
			if err := call.run(t, u, server.URL, directory); err != nil {
				t.Fatalf("request failed: %v", err)
			}
			for name, want := range map[string]string{"Accept": call.accept, "Authorization": call.authorization, "Private-Token": call.privateToken} {
				if got := server.header.Get(name); got != want {
					t.Fatalf("%s = %q, want %q", name, got, want)
				}
			}
			if call.document == nil {
				assertDownloadedAsset(t, filepath.Join(directory, "asset.tar.gz"), body)
			}

			server.answer(http.StatusTeapot, "")
			if err := call.run(t, u, server.URL, t.TempDir()); err == nil || err.Error() != call.status || isSourceUnavailable(err) {
				t.Fatalf("418 failure = %v, want %q", err, call.status)
			}
			server.answer(http.StatusServiceUnavailable, "")
			if err := call.run(t, u, server.URL, t.TempDir()); err == nil || !isSourceUnavailable(err) || !strings.HasSuffix(err.Error(), ": 503 Service Unavailable") {
				t.Fatalf("503 failure = %v, want an unavailable source", err)
			}
			if err := call.run(t, Updater{HTTPClient: &http.Client{}}, closedListenerURL(t), t.TempDir()); err == nil || !isSourceUnavailable(err) || !strings.HasPrefix(err.Error(), call.transfer+`"`) {
				t.Fatalf("transfer failure = %v, want it to start with %q", err, call.transfer)
			}
			if call.parse != "" {
				server.answer(http.StatusOK, "<html>")
				if err := call.run(t, u, server.URL, t.TempDir()); err == nil || err.Error() != call.parse {
					t.Fatalf("parse failure = %v, want %q", err, call.parse)
				}
			}
		})
	}
}

// A metadata request decodes at most its limit: a document that fits is read,
// and a longer one is cut short.
func TestReleaseMetadataRequestsKeepTheirLimits(t *testing.T) {
	t.Setenv("AIGW_GITHUB_TOKEN", "github-fixture")
	t.Setenv("GITLAB_TOKEN", "gitlab-fixture")
	for _, call := range releaseCalls() {
		if call.document == nil {
			continue
		}
		t.Run(call.name, func(t *testing.T) {
			server := newReleaseServer(t)
			u := Updater{HTTPClient: server.Client()}
			server.answer(http.StatusOK, call.document(call.limit))
			if err := call.run(t, u, server.URL, t.TempDir()); err != nil {
				t.Fatalf("document of %d bytes failed: %v", call.limit, err)
			}
			server.answer(http.StatusOK, call.document(call.limit+1))
			if err := call.run(t, u, server.URL, t.TempDir()); err == nil || !strings.HasSuffix(err.Error(), ": unexpected EOF") {
				t.Fatalf("document of %d bytes = %v, want it cut short", call.limit+1, err)
			}
		})
	}
}

func assertDownloadedAsset(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("downloaded asset = %q, %v; want %q", got, err, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("downloaded asset mode = %o, want 600", info.Mode().Perm())
	}
}
