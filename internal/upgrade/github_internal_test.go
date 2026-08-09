package upgrade

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type githubRoundTripFunc func(*http.Request) (*http.Response, error)

func githubNotFoundError() error {
	return releaseHTTPError{provider: ReleaseProviderGitHub, operation: "query latest release", statusCode: http.StatusNotFound, status: "404 Not Found"}
}

func (fn githubRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func closedListenerURL(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return "https://" + address
}

func TestLatestPrereleaseTagFromGitHubReturnsOriginalErrorWhenNotNotFound(t *testing.T) {
	u := Updater{}
	original := releaseHTTPError{provider: ReleaseProviderGitHub, operation: "query latest release", statusCode: http.StatusInternalServerError, status: "500 Internal Server Error"}
	if _, err := u.latestPrereleaseTagFromGitHub(context.Background(), ReleaseSource{}, original); err != original {
		t.Fatalf("error = %v, want %v", err, original)
	}
}

func TestLatestPrereleaseTagFromGitHubDoesNotTreatIncidental404TextAsNotFound(t *testing.T) {
	u := Updater{}
	original := errors.New("transport failed after 404 retries")
	if _, err := u.latestPrereleaseTagFromGitHub(context.Background(), ReleaseSource{}, original); err != original {
		t.Fatalf("error = %v, want original error identity", err)
	}
}

func TestLatestPrereleaseTagFromGitHubRejectsMalformedRepository(t *testing.T) {
	u := Updater{}
	source := ReleaseSource{Origin: "https://api.github.com", Repository: "o/r\n"}
	latestErr := releaseHTTPError{provider: ReleaseProviderGitHub, operation: "query latest release", statusCode: http.StatusNotFound, status: "404 Not Found"}
	if _, err := u.latestPrereleaseTagFromGitHub(context.Background(), source, latestErr); err == nil || !strings.Contains(err.Error(), "create GitHub prerelease metadata request") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestPrereleaseTagFromGitHubPropagatesAuthorizationFailure(t *testing.T) {
	t.Setenv("AIGW_GITHUB_TOKEN", "bad\ntoken")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	u := Updater{}
	source := ReleaseSource{Origin: "https://api.github.com", Repository: "o/r"}
	if _, err := u.latestPrereleaseTagFromGitHub(context.Background(), source, githubNotFoundError()); err == nil || !strings.Contains(err.Error(), "control character") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestPrereleaseTagFromGitHubReportsUnavailableOnConnectionFailure(t *testing.T) {
	u := Updater{HTTPClient: &http.Client{}}
	source := ReleaseSource{Origin: closedListenerURL(t), Repository: "o/r"}
	_, err := u.latestPrereleaseTagFromGitHub(context.Background(), source, githubNotFoundError())
	if err == nil || !isGitHubUnavailable(err) {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func TestLatestPrereleaseTagFromGitHubReportsUnavailableOnRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	_, err := u.latestPrereleaseTagFromGitHub(context.Background(), source, githubNotFoundError())
	if err == nil || !isGitHubUnavailable(err) {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func TestLatestPrereleaseTagFromGitHubReturnsOriginalErrorOnOtherFailureStatus(t *testing.T) {
	original := githubNotFoundError()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	if _, err := u.latestPrereleaseTagFromGitHub(context.Background(), source, original); err != original {
		t.Fatalf("error = %v, want %v", err, original)
	}
}

func TestLatestPrereleaseTagFromGitHubRejectsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	if _, err := u.latestPrereleaseTagFromGitHub(context.Background(), source, githubNotFoundError()); err == nil || !strings.Contains(err.Error(), "parse GitHub prerelease metadata") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestPrereleaseTagFromGitHubReturnsOriginalErrorWhenNoPublishedPrerelease(t *testing.T) {
	original := githubNotFoundError()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"v1.0.0","prerelease":false,"published_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	if _, err := u.latestPrereleaseTagFromGitHub(context.Background(), source, original); err != original {
		t.Fatalf("error = %v, want %v", err, original)
	}
}

func TestLatestPublishedGitHubPrereleaseSkipsIneligibleEntriesAndPicksNewest(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v1.0.0-rc.1", Prerelease: true, Draft: true, PublishedAt: "2026-01-01T00:00:00Z"},
		{TagName: "v1.0.0", Prerelease: false, PublishedAt: "2026-01-01T00:00:00Z"},
		{TagName: "v1.0.0-rc.2", Prerelease: true, PublishedAt: ""},
		{TagName: "not-a-version", Prerelease: true, PublishedAt: "2026-01-01T00:00:00Z"},
		{TagName: "v1.0.0-rc.1", Prerelease: true, PublishedAt: "2026-01-01T00:00:00Z"},
		{TagName: "v1.0.0-rc.3", Prerelease: true, PublishedAt: "2026-01-02T00:00:00Z"},
	}
	got := latestPublishedGitHubPrerelease(releases)
	if got != "v1.0.0-rc.3" {
		t.Fatalf("latest = %q", got)
	}
}

func TestLatestTagFromGitHubReleaseReturnsOriginalErrorForNonNotFoundFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	_, err := u.latestTagFromGitHubRelease(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitHubReleaseReturnsPrereleaseErrorWhenCLIFallbackFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			http.NotFound(w, r)
		case "/repos/o/r/releases":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client(), Runner: &fakeCommandRunner{fail: map[string]error{"gh": errors.New("gh missing")}}}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	_, err := u.latestTagFromGitHubRelease(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitHubReleaseRejectsEmptyTagName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":""}`))
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	if _, err := u.latestTagFromGitHubRelease(context.Background(), source); err == nil || !strings.Contains(err.Error(), "no AIGW release is available") {
		t.Fatalf("error = %v", err)
	}
}

func TestGithubReleaseRejectsMalformedRepository(t *testing.T) {
	u := Updater{}
	source := ReleaseSource{Origin: "https://api.github.com", Repository: "o/r\n"}
	if _, err := u.githubRelease(context.Background(), source, "releases/latest"); err == nil || !strings.Contains(err.Error(), "create GitHub release metadata request") {
		t.Fatalf("error = %v", err)
	}
}

func TestGithubReleasePropagatesAuthorizationFailure(t *testing.T) {
	t.Setenv("AIGW_GITHUB_TOKEN", "bad\ntoken")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	u := Updater{}
	source := ReleaseSource{Origin: "https://api.github.com", Repository: "o/r"}
	if _, err := u.githubRelease(context.Background(), source, "releases/latest"); err == nil || !strings.Contains(err.Error(), "control character") {
		t.Fatalf("error = %v", err)
	}
}

func TestGithubReleaseReportsUnavailableOnConnectionFailure(t *testing.T) {
	u := Updater{HTTPClient: &http.Client{}}
	source := ReleaseSource{Origin: closedListenerURL(t), Repository: "o/r"}
	if _, err := u.githubRelease(context.Background(), source, "releases/latest"); err == nil || !isGitHubUnavailable(err) {
		t.Fatalf("error = %v", err)
	}
}

func TestGithubReleaseReportsUnavailableOnServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	if _, err := u.githubRelease(context.Background(), source, "releases/latest"); err == nil || !isGitHubUnavailable(err) {
		t.Fatalf("error = %v", err)
	}
}

func TestGithubReleaseRejectsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	if _, err := u.githubRelease(context.Background(), source, "releases/latest"); err == nil || !strings.Contains(err.Error(), "parse GitHub release metadata") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsSkipsInvalidMetadataEntries(t *testing.T) {
	release := githubRelease{}
	release.Assets = append(release.Assets, struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{Name: "nested/name", BrowserDownloadURL: "https://example.test/a"})
	release.Assets = append(release.Assets, struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{Name: "asset.tar.gz", BrowserDownloadURL: ""})
	u := Updater{}
	if err := u.downloadGitHubReleaseAssets(context.Background(), release, t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "does not include") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsRejectsInvalidRequestedName(t *testing.T) {
	u := Updater{}
	if err := u.downloadGitHubReleaseAssets(context.Background(), githubRelease{}, t.TempDir(), "nested/name"); err == nil || !strings.Contains(err.Error(), "invalid release asset name") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsPropagatesDownloadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	release := githubRelease{}
	release.Assets = append(release.Assets, struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{Name: "asset.tar.gz", BrowserDownloadURL: server.URL + "/asset.tar.gz"})
	u := Updater{HTTPClient: server.Client()}
	if err := u.downloadGitHubReleaseAssets(context.Background(), release, t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "download GitHub release asset") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsFromGitHubPropagatesNonNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	if err := u.downloadReleaseAssetsFromGitHub(context.Background(), source, "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsFromGitHubRejectsTagMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0"}`))
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Origin: server.URL, Repository: "o/r"}
	err := u.downloadReleaseAssetsFromGitHub(context.Background(), source, "v1.0.0", t.TempDir(), "asset.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "does not match requested tag") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsFromGitHubFallsBackToCLIOn404(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: githubRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		want := "https://api.github.com/repos/o/r/releases/tags/v1.0.0"
		if request.Method != http.MethodGet || request.URL.String() != want {
			return nil, fmt.Errorf("unexpected GitHub request: %s %s", request.Method, request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     "404 Not Found",
			Body:       http.NoBody,
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	directory := t.TempDir()
	u := Updater{HTTPClient: client, Runner: &writingCommandRunner{directory: directory}}
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	if err := u.downloadReleaseAssetsFromGitHub(context.Background(), source, "v1.0.0", directory, "asset.tar.gz"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("GitHub API requests = %d, want one controlled request", requests)
	}
}

func TestDownloadGitHubAssetRejectsMalformedURL(t *testing.T) {
	u := Updater{}
	if err := u.downloadGitHubAsset(context.Background(), "http://[::1]:invalid-port/a", filepath.Join(t.TempDir(), "a")); err == nil || !strings.Contains(err.Error(), "create GitHub asset request") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubAssetPropagatesAuthorizationFailure(t *testing.T) {
	t.Setenv("AIGW_GITHUB_TOKEN", "bad\ntoken")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	u := Updater{}
	if err := u.downloadGitHubAsset(context.Background(), "https://example.test/a", filepath.Join(t.TempDir(), "a")); err == nil || !strings.Contains(err.Error(), "control character") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubAssetReportsUnavailableOnConnectionFailure(t *testing.T) {
	u := Updater{HTTPClient: &http.Client{}}
	if err := u.downloadGitHubAsset(context.Background(), closedListenerURL(t)+"/a", filepath.Join(t.TempDir(), "a")); err == nil || !isGitHubUnavailable(err) {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubAssetReportsUnavailableOnServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	if err := u.downloadGitHubAsset(context.Background(), server.URL+"/a", filepath.Join(t.TempDir(), "a")); err == nil || !isGitHubUnavailable(err) {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubAssetRejectsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	if err := u.downloadGitHubAsset(context.Background(), server.URL+"/a", filepath.Join(t.TempDir(), "a")); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubAssetRejectsUnwritableDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	missing := filepath.Join(t.TempDir(), "missing", "a")
	if err := u.downloadGitHubAsset(context.Background(), server.URL+"/a", missing); err == nil || !strings.Contains(err.Error(), "create downloaded asset") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubAssetWritesFileOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	destination := filepath.Join(t.TempDir(), "a")
	if err := u.downloadGitHubAsset(context.Background(), server.URL+"/a", destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "payload" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
