package upgrade

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReleaseHTTPClientStripsCredentialsAcrossOriginChain(t *testing.T) {
	forwarded := make(chan http.Header, 2)
	var originURL string
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded <- r.Header.Clone()
		http.Redirect(w, r, originURL+"/asset", http.StatusFound)
	}))
	defer target.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			forwarded <- r.Header.Clone()
			return
		}
		http.Redirect(w, r, target.URL+"/asset", http.StatusFound)
	}))
	defer origin.Close()
	originURL = origin.URL
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	roots.AddCert(target.Certificate())
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	t.Cleanup(transport.CloseIdleConnections)
	u := Updater{HTTPClient: &http.Client{Transport: transport}}
	client := u.releaseHTTPClient()
	request, err := http.NewRequest(http.MethodGet, origin.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Private-Token", "secret")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	for range 2 {
		headers := <-forwarded
		for _, name := range []string{"Authorization", "PRIVATE-TOKEN"} {
			if got := headers.Get(name); got != "" {
				t.Fatalf("%s forwarded across origins: %q", name, got)
			}
		}
	}
}

func TestReleaseHTTPClientPreservesNativeRedirectBound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step, _ := strconv.Atoi(r.URL.Query().Get("step"))
		if step < 12 {
			http.Redirect(w, r, fmt.Sprintf("/?step=%d", step+1), http.StatusFound)
		}
	}))
	defer server.Close()
	base := server.Client()
	for _, client := range []*http.Client{base, (Updater{HTTPClient: base}).releaseHTTPClient()} {
		response, err := client.Get(server.URL)
		if response != nil {
			_ = response.Body.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
			t.Errorf("native redirect bound: %v", err)
		}
	}
}

func TestReleaseHTTPClientUsesSchemeInOrigin(t *testing.T) {
	for _, target := range []string{"http://release.example.test", "https://release.example.test"} {
		t.Run(target, func(t *testing.T) {
			previous, err := http.NewRequest(http.MethodGet, "http://release.example.test", nil)
			if err != nil {
				t.Fatal(err)
			}
			request, err := http.NewRequest(http.MethodGet, target, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer secret")
			request.Header.Set("Private-Token", "secret")
			want := request.Header.Clone()
			if request.URL.Scheme != previous.URL.Scheme {
				want = make(http.Header)
			}
			client := (Updater{}).releaseHTTPClient()
			if err := client.CheckRedirect(request, []*http.Request{previous}); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"Authorization", "Private-Token"} {
				if request.Header.Get(name) != want.Get(name) {
					t.Errorf("%s = %q, want %q", name, request.Header.Get(name), want.Get(name))
				}
			}
		})
	}
}

func TestReleaseHTTPClientRejectsHTTPSDowngradeRedirect(t *testing.T) {
	plainCalled := make(chan struct{}, 1)
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plainCalled <- struct{}{}
	}))
	defer plain.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+"/asset", http.StatusFound)
	}))
	defer origin.Close()
	u := Updater{HTTPClient: origin.Client()}
	client := u.releaseHTTPClient()
	response, err := client.Get(origin.URL)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "HTTPS to HTTP") {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-plainCalled:
		t.Fatal("client followed an HTTPS-to-HTTP downgrade redirect")
	default:
	}
}

func TestReleaseHTTPClientRejectsDowngradeAfterHTTPOrigin(t *testing.T) {
	client := (Updater{}).releaseHTTPClient()
	request := &http.Request{URL: &url.URL{Scheme: "http", Host: "release.example.test"}}
	previous := []*http.Request{
		{URL: &url.URL{Scheme: "http", Host: "release.example.test"}},
		{URL: &url.URL{Scheme: "https", Host: "release.example.test"}},
	}
	if err := client.CheckRedirect(request, previous); err == nil || !strings.Contains(err.Error(), "HTTPS to HTTP") {
		t.Fatalf("downgrade after HTTPS hop: %v", err)
	}
}

func TestReleaseHTTPClientChainsExistingCheckRedirect(t *testing.T) {
	called := false
	base := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		called = true
		return nil
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	u := Updater{HTTPClient: base}
	client := u.releaseHTTPClient()
	response, err := client.Get(server.URL + "/start")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if !called {
		t.Fatal("existing CheckRedirect was not invoked")
	}
}

func TestReleaseHTTPClientDefaultsClientWhenUnset(t *testing.T) {
	u := Updater{}
	client := u.releaseHTTPClient()
	if client.Timeout != releaseRequestTimeout {
		t.Fatalf("timeout = %v, want %v", client.Timeout, releaseRequestTimeout)
	}
}

func TestUpdateWithNilRunnerRequiresConfiguredSource(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, name := range []string{
		"AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY",
		"AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY",
	} {
		t.Setenv(name, "")
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	if _, err := u.Update(context.Background(), "0.1.0"); err == nil || !strings.Contains(err.Error(), "release source is not configured") {
		t.Fatalf("unconfigured source error = %v", err)
	}
}

func TestUpdateRejectsUnwritableWorkspace(t *testing.T) {
	// os.MkdirTemp("", ...) resolves the base directory through
	// os.TempDir(), which honors different environment variables per OS:
	// $TMPDIR on Unix, but %TMP%/%TEMP%/%USERPROFILE% (in that order) on
	// Windows. All of them must be pointed at a missing directory so the
	// workspace creation fails deterministically on every platform.
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("TMPDIR", missing)
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	u := Updater{Runner: &recordingRunner{}}
	releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitLab}, Tag: "v1.0.0"}}
	if _, err := u.updateFromResolvedPeers(t.Context(), releases, "0.1.0"); err == nil || !strings.Contains(err.Error(), "create update workspace") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateOwnsDownloadWorkspace(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			temporary, installation := t.TempDir(), t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, temporary)
			}
			executable := filepath.Join(installation, "aigw")
			for path, content := range map[string]string{
				executable: "current", rollbackPath(executable): "previous",
				filepath.Join(temporary, "foreign"): "preserved",
			} {
				if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			archive := tarGzForTest(t, "aigw_1.0.0_darwin_arm64/aigw", []byte("candidate"))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				w.WriteHeader(status)
				if strings.Contains(request.URL.Path, "/releases/tags/") {
					_, _ = fmt.Fprintf(w, `{"tag_name":"v1.0.0","assets":[{"name":"aigw_1.0.0_darwin_arm64.tar.gz","browser_download_url":"http://%[1]s/archive"},{"name":"checksums.txt","browser_download_url":"http://%[1]s/checksums.txt"}]}`, request.Host)
					return
				}
				if strings.HasSuffix(request.URL.Path, "/checksums.txt") {
					_, _ = fmt.Fprintf(w, "%x  aigw_1.0.0_darwin_arm64.tar.gz\n", sha256.Sum256(archive))
					return
				}
				_, _ = w.Write(archive)
			}))
			defer server.Close()
			u := Updater{
				GOOS: "darwin", GOARCH: "arm64", Executable: executable,
				Runner: &recordingRunner{output: []byte("aigw version 1.0.0\n")}, HTTPClient: server.Client(),
			}
			releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitHub, Origin: server.URL, Repository: "team/product"}, Tag: "v1.0.0"}}
			_, err := u.updateFromResolvedPeers(t.Context(), releases, "0.1.0")
			if (err == nil) != (status == http.StatusOK) {
				t.Fatalf("update outcome for HTTP %d: %v", status, err)
			}
			current, previous := "current", "previous"
			if status == http.StatusOK {
				current, previous = "candidate", "current"
			}
			for path, want := range map[string]string{
				executable: current, rollbackPath(executable): previous,
				filepath.Join(temporary, "foreign"): "preserved",
			} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatalf("owned cleanup changed %s: %q, %v; want %q", path, got, err, want)
				}
			}
			entries, err := os.ReadDir(temporary)
			if err != nil || len(entries) != 1 || entries[0].Name() != "foreign" {
				t.Fatalf("update workspace cleanup left %v: %v", entries, err)
			}
		})
	}
}

func TestDownloadPeerAssetsPropagatesNonUnavailableFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{
		HTTPClient: server.Client(),
		Runner:     &recordingRunner{err: errors.New("permission denied")},
		GitLab:     ReleaseSource{Origin: server.URL, Repository: "group/project"},
	}
	releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitLab, Origin: server.URL, Repository: "group/project"}, Tag: "v1.0.0"}}
	if _, err := u.downloadPeerAssets(context.Background(), releases, "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "release assets failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadPeerAssetsFailsWhenAllSourcesAreUnavailable(t *testing.T) {
	u := Updater{Runner: &recordingRunner{err: exec.ErrNotFound}}
	releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitLab}, Tag: "v1.0.0"}}
	_, err := u.downloadPeerAssets(context.Background(), releases, "asset.tar.gz", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "all reachable release sources failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestUnavailablePeersPreserveEveryCause(t *testing.T) {
	gitlabFailure := errors.New("GitLab transport failure")
	githubFailure := errors.New("GitHub transport failure")
	t.Setenv("GITLAB_TOKEN", "fixture-token")
	u := Updater{
		Runner: &recordingRunner{err: exec.ErrNotFound},
		HTTPClient: &http.Client{Transport: githubRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Host == "gitlab.test" {
				return nil, gitlabFailure
			}
			return nil, githubFailure
		})},
	}
	sources := []ReleaseSource{
		{Provider: ReleaseProviderGitLab, Origin: "https://gitlab.test", Repository: "team/product"},
		{Provider: ReleaseProviderGitHub, Origin: "https://github.test", Repository: "team/product"},
	}
	t.Run("metadata", func(t *testing.T) {
		_, err := u.resolvePeerReleases(t.Context(), sources...)
		if !errors.Is(err, gitlabFailure) || !errors.Is(err, githubFailure) {
			t.Fatalf("peer metadata error lost a cause: %v", err)
		}
	})
	t.Run("assets", func(t *testing.T) {
		releases := []resolvedRelease{{Source: sources[0], Tag: "v1.0.0"}, {Source: sources[1], Tag: "v1.0.0"}}
		_, err := u.downloadPeerAssets(t.Context(), releases, "asset.tar.gz", t.TempDir())
		if !errors.Is(err, gitlabFailure) || !errors.Is(err, githubFailure) {
			t.Fatalf("peer asset error lost a cause: %v", err)
		}
	})
}
