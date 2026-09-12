package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type erroringReader struct{}

func (erroringReader) Read([]byte) (int, error) { return 0, errors.New("simulated read failure") }

type brokenBodyRoundTripper struct{ status int }

func (r brokenBodyRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(erroringReader{}),
		Header:     make(http.Header),
	}, nil
}

func TestDownloadReleaseAssetsReturnsGlabUnavailableTokenFallbackFailure(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{Runner: &recordingRunner{err: fmt.Errorf("glab lookup: %w", exec.ErrNotFound)}}
	err := u.downloadReleaseAssets(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "authenticated glab fallback failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestGlabAvailabilityDoesNotDependOnErrorText(t *testing.T) {
	if isGlabUnavailable(errors.New("authenticated glab asset download is unavailable")) {
		t.Fatal("plain error text was misclassified as source unavailability")
	}
	if !isGlabUnavailable(unavailable(errors.New("glab unavailable with arbitrary text"))) {
		t.Fatal("typed unavailability was not recognized")
	}
}

func TestDownloadReleaseAssetsWrapsFailureWhenNonUnavailableAndTokenInvalid(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{Runner: &recordingRunner{err: errors.New("glab exited with an unexpected error")}}
	err := u.downloadReleaseAssets(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "authenticated glab fallback failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIRejectsMalformedOrigin(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{GitLab: ReleaseSource{Origin: "https://example.test/bad\nhost", Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "create GitLab release-download request") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIReportsConnectionFailure(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: &http.Client{}, GitLab: ReleaseSource{Origin: closedListenerURL(t), Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "download release asset") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIPropagatesCopyFailure(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: &http.Client{Transport: brokenBodyRoundTripper{}}, GitLab: ReleaseSource{Origin: "https://example.test", Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "write downloaded release asset") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagWrapsNonUnavailableGlabFailure(t *testing.T) {
	u := Updater{Runner: &recordingRunner{err: errors.New("permission denied")}}
	if _, err := u.latestTag(context.Background()); err == nil || !strings.Contains(err.Error(), "query latest release") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagRejectsEmptyGlabOutput(t *testing.T) {
	u := Updater{Runner: &recordingRunner{output: []byte("\n")}}
	if _, err := u.latestTag(context.Background()); err == nil || !strings.Contains(err.Error(), "no AIGW release is available") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagRejectsMalformedGlabOutput(t *testing.T) {
	runner := &recordingRunner{output: []byte("not-a-version\n")}
	u := Updater{Runner: runner}
	if _, err := u.latestTag(context.Background()); err == nil {
		t.Fatal("latestTag accepted a malformed tag")
	}
}

func TestLatestTagFallsBackToTokenWhenGlabMissing(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{Runner: &recordingRunner{err: exec.ErrNotFound}}
	if _, err := u.latestTag(context.Background()); err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN fallback requires explicit") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagCombinesGlabAndAPIUnavailability(t *testing.T) {
	url := closedListenerURL(t)
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: &http.Client{}, Runner: &recordingRunner{err: exec.ErrNotFound}, GitLab: ReleaseSource{Origin: url, Repository: "group/project"}}
	_, err := u.latestTag(context.Background())
	if err == nil || !isSourceUnavailable(err) || !strings.Contains(err.Error(), "GitLab release lookup failed through glab and API") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagReturnsAPIResultWhenGlabUnavailableButAPISucceeds(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: server.Client(), Runner: &recordingRunner{err: exec.ErrNotFound}, GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	tag, err := u.latestTag(context.Background())
	if err != nil || tag != "v1.2.3" {
		t.Fatalf("tag=%q err=%v", tag, err)
	}
}

func TestReleaseTagFromCLIOutputPicksLastNonEmptyLine(t *testing.T) {
	tag, err := releaseTagFromCLIOutput([]byte("Warning: config\n\nv1.2.3\n"))
	if err != nil || tag != "v1.2.3" {
		t.Fatalf("tag=%q err=%v", tag, err)
	}
}

func TestReleaseTagFromCLIOutputReturnsEmptyForBlankOutput(t *testing.T) {
	tag, err := releaseTagFromCLIOutput([]byte("\n \n"))
	if err != nil || tag != "" {
		t.Fatalf("tag=%q err=%v", tag, err)
	}
}

func TestReleaseTagFromCLIOutputRejectsMalformedFinalTag(t *testing.T) {
	if _, err := releaseTagFromCLIOutput([]byte("not-a-version\n")); err == nil {
		t.Fatal("releaseTagFromCLIOutput accepted a malformed tag")
	}
}

func TestLatestTagFromGitLabAPIRequiresToken(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{}
	if _, err := u.latestTagFromGitLabAPI(context.Background()); err == nil {
		t.Fatal("latestTagFromGitLabAPI accepted a missing token")
	}
}

func TestLatestTagFromGitLabAPIRejectsMalformedOrigin(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{GitLab: ReleaseSource{Origin: "https://example.test/bad\nhost", Repository: "group/project"}}
	if _, err := u.latestTagFromGitLabAPI(context.Background()); err == nil || !strings.Contains(err.Error(), "create GitLab latest-release request") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitLabAPIReportsUnavailableOnConnectionFailure(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	url := closedListenerURL(t)
	u := Updater{HTTPClient: &http.Client{}, GitLab: ReleaseSource{Origin: url, Repository: "group/project"}}
	_, err := u.latestTagFromGitLabAPI(context.Background())
	if err == nil || !isSourceUnavailable(err) {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitLabAPIReportsUnavailableOnRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	_, err := u.latestTagFromGitLabAPI(context.Background())
	if err == nil || !isSourceUnavailable(err) {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitLabAPIRejectsOtherFailureStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	_, err := u.latestTagFromGitLabAPI(context.Background())
	if err == nil || isSourceUnavailable(err) || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v", err)
	}
	var httpErr releaseHTTPError
	if !errors.As(err, &httpErr) || httpErr.statusCode != http.StatusForbidden || httpErr.provider != ReleaseProviderGitLab {
		t.Fatalf("error = %#v, want typed GitLab 403", err)
	}
}

func TestLatestTagFromGitLabAPIReportsUnusableReleaseMetadata(t *testing.T) {
	for _, test := range []struct {
		name, payload, problem string
	}{
		{"malformed JSON", "not-json", "parse GitLab latest release"},
		{"empty tag", `{"tag_name":""}`, "no AIGW release is available"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(test.payload))
			}))
			defer server.Close()
			t.Setenv("GITLAB_TOKEN", "token")
			u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
			if _, err := u.latestTagFromGitLabAPI(t.Context()); err == nil || !strings.Contains(err.Error(), test.problem) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateTokenFallbackHostRejectsEmptyOrigin(t *testing.T) {
	u := Updater{}
	if err := u.validateTokenFallbackHost(); err == nil || !strings.Contains(err.Error(), "requires explicit") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateTokenFallbackHostRejectsNonHTTPSOrigin(t *testing.T) {
	u := Updater{GitLab: ReleaseSource{Origin: "http://example.test"}}
	if err := u.validateTokenFallbackHost(); err == nil || !strings.Contains(err.Error(), "HTTPS origin") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateTokenFallbackHostRejectsOriginWithCredentialsOrQuery(t *testing.T) {
	cases := []string{
		"https://user:pass@example.test",
		"https://example.test?x=1",
		"https://example.test#frag",
		"https://example.test/path",
	}
	for _, origin := range cases {
		u := Updater{GitLab: ReleaseSource{Origin: origin}}
		if err := u.validateTokenFallbackHost(); err == nil {
			t.Fatalf("origin=%q: accepted an invalid fallback host", origin)
		}
	}
}

func TestValidateTokenFallbackHostAcceptsPlainHTTPSOrigin(t *testing.T) {
	u := Updater{GitLab: ReleaseSource{Origin: "https://example.test"}}
	if err := u.validateTokenFallbackHost(); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseAssetNameRejectsUnparsableURL(t *testing.T) {
	if got := releaseAssetName("http://[::1]:invalid-port/name"); got != "" {
		t.Fatalf("releaseAssetName = %q", got)
	}
}

func TestReleaseAssetNameRejectsEmptyOrRootPath(t *testing.T) {
	if got := releaseAssetName("https://example.test"); got != "" {
		t.Fatalf("releaseAssetName(no path) = %q", got)
	}
	if got := releaseAssetName("https://example.test/"); got != "" {
		t.Fatalf("releaseAssetName(root path) = %q", got)
	}
}

func TestReleaseAssetNameReturnsBaseName(t *testing.T) {
	if got := releaseAssetName("https://example.test/downloads/aigw_1.2.3.tar.gz"); got != "aigw_1.2.3.tar.gz" {
		t.Fatalf("releaseAssetName = %q", got)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIRejectsInvalidAssetName(t *testing.T) {
	u := Updater{}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "nested/name", t.TempDir()); err == nil {
		t.Fatal("downloadReleaseAssetFromGitLabAPI accepted a nested asset name")
	}
}

func TestDownloadReleaseAssetFromGitLabAPIRequiresToken(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil {
		t.Fatal("downloadReleaseAssetFromGitLabAPI accepted a missing token")
	}
}

func TestDownloadReleaseAssetFromGitLabAPIRejectsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "group/project")
	u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIRejectsUnwritableDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "group/project")
	u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	missingDirectory := filepath.Join(t.TempDir(), "missing")
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", missingDirectory); err == nil {
		t.Fatal("downloadReleaseAssetFromGitLabAPI accepted an unwritable destination")
	}
}

func TestDownloadReleaseAssetFromGitLabAPIWritesFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "group/project")
	directory := t.TempDir()
	u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", directory); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(directory, "asset.tar.gz"))
	if err != nil || string(got) != "payload" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
