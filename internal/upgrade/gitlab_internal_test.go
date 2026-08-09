package upgrade

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
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

func TestDownloadReleaseAssetsFromExactSourceRejectsUnsupportedProvider(t *testing.T) {
	u := Updater{}
	unavailableFlag, err := u.downloadReleaseAssetsFromExactSource(context.Background(), ReleaseSource{Provider: "bogus"}, "v1.0.0", t.TempDir(), "asset.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "unsupported release provider") {
		t.Fatalf("error = %v", err)
	}
	if unavailableFlag {
		t.Fatal("unsupported provider incorrectly reported as unavailable")
	}
}

type glabDownloadFailRunner struct {
	downloadErr error
}

func (r *glabDownloadFailRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	return nil, r.downloadErr
}

func TestDownloadReleaseAssetsReturnsGlabUnavailableTokenFallbackFailure(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{Runner: &glabDownloadFailRunner{downloadErr: &erroringExecNotFound{}}}
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{Runner: &glabDownloadFailRunner{downloadErr: errors.New("glab exited with an unexpected error")}}
	err := u.downloadReleaseAssets(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "authenticated glab fallback failed") {
		t.Fatalf("error = %v", err)
	}
}

type erroringExecNotFound struct{}

func (e *erroringExecNotFound) Error() string { return "authenticated glab command not found" }

func (e *erroringExecNotFound) Unwrap() error { return exec.ErrNotFound }

func TestDownloadReleaseAssetsWithGlabAPISkipsLinksWithEmptyURL(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":""},{"name":"asset.tar.gz","url":"https://example.test/downloads/asset.tar.gz"}]}}`
	directory := t.TempDir()
	runner := &writingFileRunner{recordingFileRunner: &recordingFileRunner{apiOutput: []byte(metadata)}, directory: directory}
	u := Updater{Runner: runner}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", directory, "asset.tar.gz"); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIRejectsMalformedOrigin(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://example.test/bad\nhost")
	u := Updater{GitLab: ReleaseSource{Origin: "https://example.test/bad\nhost", Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "create GitLab release-download request") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIReportsConnectionFailure(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", closedListenerURL(t))
	u := Updater{HTTPClient: &http.Client{}, GitLab: ReleaseSource{Origin: closedListenerURL(t), Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "download release asset") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetFromGitLabAPIPropagatesCopyFailure(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://example.test")
	u := Updater{HTTPClient: &http.Client{Transport: brokenBodyRoundTripper{}}, GitLab: ReleaseSource{Origin: "https://example.test", Repository: "group/project"}}
	if err := u.downloadReleaseAssetFromGitLabAPI(context.Background(), "v1.0.0", "asset.tar.gz", t.TempDir()); err == nil || !strings.Contains(err.Error(), "write downloaded release asset") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromSourceRejectsUnsupportedProvider(t *testing.T) {
	u := Updater{}
	_, unavailableFlag, err := u.latestTagFromSource(context.Background(), ReleaseSource{Provider: "bogus"})
	if err == nil || !strings.Contains(err.Error(), "unsupported release provider") {
		t.Fatalf("error = %v", err)
	}
	if unavailableFlag {
		t.Fatal("unsupported provider incorrectly reported as unavailable")
	}
}

func TestLatestTagFromSourceMapsGitHubUnavailability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	u := Updater{HTTPClient: server.Client()}
	source := ReleaseSource{Provider: ReleaseProviderGitHub, Origin: server.URL, Repository: "o/r"}
	_, unavailableFlag, err := u.latestTagFromSource(context.Background(), source)
	if err == nil || !unavailableFlag {
		t.Fatalf("err=%v unavailable=%v", err, unavailableFlag)
	}
}

func TestLatestTagWrapsNonUnavailableGlabFailure(t *testing.T) {
	u := Updater{Runner: &fakeCommandRunner{fail: map[string]error{"glab": errors.New("permission denied")}}}
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

type recordingRunner struct {
	output []byte
	err    error
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	return r.output, r.err
}

func TestLatestTagFallsBackToTokenWhenGlabMissing(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("GITLAB_TOKEN", "")
	u := Updater{Runner: &missingGlabRunnerInternal{}}
	if _, err := u.latestTag(context.Background()); err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN fallback requires explicit") {
		t.Fatalf("error = %v", err)
	}
}

type missingGlabRunnerInternal struct{}

func (missingGlabRunnerInternal) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, errNotFoundForTest
}

var errNotFoundForTest = &execNotFoundError{}

type execNotFoundError struct{}

func (*execNotFoundError) Error() string {
	return "authenticated glab: exec: \"glab\": executable file not found in $PATH"
}

func (*execNotFoundError) Unwrap() error { return exec.ErrNotFound }

func TestLatestTagCombinesGlabAndAPIUnavailability(t *testing.T) {
	url := closedListenerURL(t)
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", url)
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: &http.Client{}, Runner: &missingGlabRunnerInternal{}, GitLab: ReleaseSource{Origin: url, Repository: "group/project"}}
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{HTTPClient: server.Client(), Runner: &missingGlabRunnerInternal{}, GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://example.test/bad\nhost")
	u := Updater{GitLab: ReleaseSource{Origin: "https://example.test/bad\nhost", Repository: "group/project"}}
	if _, err := u.latestTagFromGitLabAPI(context.Background()); err == nil || !strings.Contains(err.Error(), "create GitLab latest-release request") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitLabAPIReportsUnavailableOnConnectionFailure(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "token")
	url := closedListenerURL(t)
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", url)
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
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
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
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

func TestLatestTagFromGitLabAPIRejectsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	if _, err := u.latestTagFromGitLabAPI(context.Background()); err == nil || !strings.Contains(err.Error(), "parse GitLab latest release") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitLabAPIRejectsEmptyTagName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":""}`))
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	u := Updater{HTTPClient: server.Client(), GitLab: ReleaseSource{Origin: server.URL, Repository: "group/project"}}
	if _, err := u.latestTagFromGitLabAPI(context.Background()); err == nil || !strings.Contains(err.Error(), "no AIGW release is available") {
		t.Fatalf("error = %v", err)
	}
}

type envRunner struct {
	calls [][]string
}

func (r *envRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunWithEnv(ctx, nil, name, args...)
}

func (r *envRunner) RunWithEnv(_ context.Context, env []string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append(append([]string{}, env...), append([]string{name}, args...)...))
	return []byte("v1.0.0\n"), nil
}

type envFileRunner struct {
	calls [][]string
}

func (r *envFileRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, errors.New("unexpected Run call")
}

func (r *envFileRunner) RunToFileWithEnv(_ context.Context, env []string, destination string, name string, args ...string) error {
	r.calls = append(r.calls, append(append([]string{destination}, env...), append([]string{name}, args...)...))
	return nil
}

func TestRunGlabToFileUsesEnvironmentFileRunnerWhenAvailable(t *testing.T) {
	runner := &envFileRunner{}
	u := Updater{Runner: runner}
	if err := u.runGlabToFile(context.Background(), "/tmp/dest", "api", "asset"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %v", runner.calls)
	}
}

func TestRunGlabToFileFallsBackToPlainFileRunner(t *testing.T) {
	runner := &recordingFileRunner{}
	u := Updater{Runner: runner}
	if err := u.runGlabToFile(context.Background(), "/tmp/dest", "api", "asset"); err != nil {
		t.Fatal(err)
	}
	if len(runner.fileCalls) != 1 {
		t.Fatalf("fileCalls = %v", runner.fileCalls)
	}
}

func TestRunGlabToFileRejectsRunnerWithoutFileSupport(t *testing.T) {
	u := Updater{Runner: &fakeCommandRunner{}}
	if err := u.runGlabToFile(context.Background(), "/tmp/dest", "api", "asset"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunGlabUsesEnvironmentRunnerWhenAvailable(t *testing.T) {
	runner := &envRunner{}
	u := Updater{Runner: runner}
	if _, err := u.runGlab(context.Background(), "release", "list"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %v", runner.calls)
	}
}

func TestValidateTokenFallbackHostRejectsEmptyOrigin(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	u := Updater{}
	if err := u.validateTokenFallbackHost(); err == nil || !strings.Contains(err.Error(), "requires explicit") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateTokenFallbackHostRejectsNonHTTPSOrigin(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "http://example.test")
	u := Updater{}
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
		t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", origin)
		u := Updater{}
		if err := u.validateTokenFallbackHost(); err == nil {
			t.Fatalf("origin=%q: accepted an invalid fallback host", origin)
		}
	}
}

func TestValidateTokenFallbackHostAcceptsPlainHTTPSOrigin(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://example.test")
	u := Updater{}
	if err := u.validateTokenFallbackHost(); err != nil {
		t.Fatal(err)
	}
}

func TestGitLabHTTPClientChainsExistingCheckRedirect(t *testing.T) {
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
	client := u.gitLabHTTPClient()
	response, err := client.Get(server.URL + "/start")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if !called {
		t.Fatal("existing CheckRedirect was not invoked")
	}
}

func TestGitLabHTTPClientDefaultsClientWhenUnset(t *testing.T) {
	u := Updater{}
	client := u.gitLabHTTPClient()
	if client.Timeout != releaseRequestTimeout {
		t.Fatalf("timeout = %v", client.Timeout)
	}
}
