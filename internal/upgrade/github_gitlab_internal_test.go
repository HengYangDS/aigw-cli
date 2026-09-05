package upgrade

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnwrapReturnsWrappedError(t *testing.T) {
	inner := errors.New("boom")
	wrapped := sourceUnavailableError{err: inner}
	if wrapped.Unwrap() != inner {
		t.Fatalf("Unwrap() = %v, want %v", wrapped.Unwrap(), inner)
	}
	if !errors.Is(wrapped, inner) {
		t.Fatal("errors.Is did not see through Unwrap")
	}
}

func TestUnavailableReturnsNilForNilError(t *testing.T) {
	if unavailable(nil) != nil {
		t.Fatal("unavailable(nil) returned a non-nil error")
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

type recordingFileRunner struct {
	apiOutput []byte
	apiErr    error
	fileErr   error
	fileCalls [][]string
}

func (r *recordingFileRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	return r.apiOutput, r.apiErr
}

func (r *recordingFileRunner) RunToFile(_ context.Context, destination, name string, args ...string) error {
	r.fileCalls = append(r.fileCalls, append([]string{destination, name}, args...))
	return r.fileErr
}

func TestDownloadReleaseAssetsWithGlabAPIRequiresFileCapableRunner(t *testing.T) {
	u := Updater{Runner: &fakeCommandRunner{}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRequiresAssets(t *testing.T) {
	u := Updater{Runner: &recordingFileRunner{}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir()); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIPropagatesQueryFailure(t *testing.T) {
	u := Updater{Runner: &recordingFileRunner{apiErr: errors.New("boom")}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsMalformedMetadata(t *testing.T) {
	u := Updater{Runner: &recordingFileRunner{apiOutput: []byte("not-json")}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "parse release metadata") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsInvalidAssetName(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/asset.tar.gz"}]}}`
	u := Updater{Runner: &recordingFileRunner{apiOutput: []byte(metadata)}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "nested/asset"); err == nil {
		t.Fatal("downloadReleaseAssetsWithGlabAPI accepted a nested asset name")
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsMissingAssetURL(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"other.tar.gz","url":"https://example.test/other.tar.gz"}]}}`
	u := Updater{Runner: &recordingFileRunner{apiOutput: []byte(metadata)}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "does not include") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIPropagatesDownloadFailure(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/asset.tar.gz"}]}}`
	u := Updater{Runner: &recordingFileRunner{apiOutput: []byte(metadata), fileErr: errors.New("download failed")}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsMissingWrittenFile(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/asset.tar.gz"}]}}`
	directory := t.TempDir()
	runner := &recordingFileRunner{apiOutput: []byte(metadata)}
	// RunToFile succeeds but never writes the destination file, simulating a
	// glab success response that did not deliver bytes at all.
	u := Updater{Runner: runner}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", directory, "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "inspect downloaded release asset") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsEmptyWrittenFile(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/asset.tar.gz"}]}}`
	directory := t.TempDir()
	runner := &emptyWritingFileRunner{recordingFileRunner: &recordingFileRunner{apiOutput: []byte(metadata)}}
	u := Updater{Runner: runner}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", directory, "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "did not write") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "asset.tar.gz")); !os.IsNotExist(err) {
		t.Fatalf("empty asset was not removed: %v", err)
	}
}

type emptyWritingFileRunner struct {
	*recordingFileRunner
}

func (r *emptyWritingFileRunner) RunToFile(ctx context.Context, destination, name string, args ...string) error {
	if err := r.recordingFileRunner.RunToFile(ctx, destination, name, args...); err != nil {
		return err
	}
	return os.WriteFile(destination, nil, 0o600)
}

func TestDownloadReleaseAssetsWithGlabAPIWritesAssetUsingLinkName(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/downloads/asset.tar.gz"}]}}`
	directory := t.TempDir()
	runner := &recordingFileRunner{apiOutput: []byte(metadata)}
	// Wrap manually since recordingFileRunner does not touch the filesystem.
	writerRunner := &writingFileRunner{recordingFileRunner: runner, directory: directory}
	u := Updater{}
	u.Runner = writerRunner
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", directory, "asset.tar.gz"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(directory, "asset.tar.gz"))
	if err != nil || string(got) != "downloaded" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

type writingFileRunner struct {
	*recordingFileRunner
	directory string
}

func (r *writingFileRunner) RunToFile(ctx context.Context, destination, name string, args ...string) error {
	if err := r.recordingFileRunner.RunToFile(ctx, destination, name, args...); err != nil {
		return err
	}
	return os.WriteFile(destination, []byte("downloaded"), 0o600)
}

func TestAuthorizeGitHubRequestPrefersConfiguredTokenOrder(t *testing.T) {
	t.Setenv("AIGW_GITHUB_TOKEN", "aigw-token")
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GH_TOKEN", "gh-token")
	u := &Updater{}
	request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.authorizeGitHubRequest(request); err != nil {
		t.Fatal(err)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer aigw-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestAuthorizeGitHubRequestFallsBackThroughTokenNames(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"GITHUB_TOKEN", "Bearer github-token"},
		{"GH_TOKEN", "Bearer gh-token"},
	}
	for _, tc := range cases {
		t.Setenv("AIGW_GITHUB_TOKEN", "")
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")
		value := "github-token"
		if tc.name == "GH_TOKEN" {
			value = "gh-token"
		}
		t.Setenv(tc.name, value)
		u := &Updater{}
		request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := u.authorizeGitHubRequest(request); err != nil {
			t.Fatal(err)
		}
		if got := request.Header.Get("Authorization"); got != tc.want {
			t.Fatalf("name=%s Authorization = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestAuthorizeGitHubRequestLeavesHeaderUnsetWithoutToken(t *testing.T) {
	t.Setenv("AIGW_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	u := &Updater{}
	request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.authorizeGitHubRequest(request); err != nil {
		t.Fatal(err)
	}
	if got := request.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
}

func TestAuthorizeGitHubRequestRejectsControlCharacterInEachTokenName(t *testing.T) {
	for _, name := range []string{"AIGW_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("AIGW_GITHUB_TOKEN", "")
			t.Setenv("GITHUB_TOKEN", "")
			t.Setenv("GH_TOKEN", "")
			t.Setenv(name, "bad\ntoken")
			u := &Updater{}
			request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := u.authorizeGitHubRequest(request); err == nil || !strings.Contains(err.Error(), "control character") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestReleaseHTTPClientStripsCredentialsOnCrossHostRedirect(t *testing.T) {
	forwarded := make(chan http.Header, 1)
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded <- r.Header.Clone()
	}))
	defer target.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/asset", http.StatusFound)
	}))
	defer origin.Close()
	transport := origin.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = target.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	u := Updater{HTTPClient: &http.Client{Transport: transport}}
	client := u.releaseHTTPClient()
	request, err := http.NewRequest(http.MethodGet, origin.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("PRIVATE-TOKEN", "secret")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	headers := <-forwarded
	for _, name := range []string{"Authorization", "PRIVATE-TOKEN"} {
		if got := headers.Get(name); got != "" {
			t.Fatalf("%s forwarded across hosts: %q", name, got)
		}
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
	_, err := client.Get(origin.URL)
	if err == nil || !strings.Contains(err.Error(), "HTTPS to HTTP") {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-plainCalled:
		t.Fatal("client followed an HTTPS-to-HTTP downgrade redirect")
	default:
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

type githubCLIRunner struct {
	latestOutput []byte
	latestErr    error
	listOutput   []byte
	listErr      error
	calls        [][]string
}

func (r *githubCLIRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "api" && strings.HasSuffix(args[1], "releases/latest") {
		return r.latestOutput, r.latestErr
	}
	if len(args) >= 2 && args[0] == "api" && strings.Contains(args[1], "releases?per_page=100") {
		return r.listOutput, r.listErr
	}
	return nil, errors.New("unexpected gh invocation")
}

func TestGithubReleaseWithCLIPropagatesRunError(t *testing.T) {
	u := Updater{Runner: &githubCLIRunner{latestErr: errors.New("boom")}}
	if _, err := u.githubReleaseWithCLI(context.Background(), ReleaseSource{Repository: "o/r"}, "releases/latest"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestGithubReleaseWithCLIRejectsMalformedJSON(t *testing.T) {
	u := Updater{Runner: &githubCLIRunner{latestOutput: []byte("not-json")}}
	if _, err := u.githubReleaseWithCLI(context.Background(), ReleaseSource{Repository: "o/r"}, "releases/latest"); err == nil || !strings.Contains(err.Error(), "parse GitHub release metadata through gh") {
		t.Fatalf("error = %v", err)
	}
}

func TestGithubReleaseWithCLIReturnsDecodedRelease(t *testing.T) {
	u := Updater{Runner: &githubCLIRunner{latestOutput: []byte(`{"tag_name":"v1.2.3"}`)}}
	release, err := u.githubReleaseWithCLI(context.Background(), ReleaseSource{Repository: "o/r"}, "releases/latest")
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v1.2.3" {
		t.Fatalf("tag = %q", release.TagName)
	}
}

func TestLatestTagFromGitHubCLIRejectsDisallowedSource(t *testing.T) {
	u := Updater{}
	source := ReleaseSource{Origin: "https://example.test", Repository: "o/r"}
	if _, err := u.latestTagFromGitHubCLI(context.Background(), source); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitHubCLIRejectsEmptyLatestTag(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &githubCLIRunner{latestOutput: []byte(`{"tag_name":""}`)}}
	if _, err := u.latestTagFromGitHubCLI(context.Background(), source); err == nil || !strings.Contains(err.Error(), "no AIGW release is available") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitHubCLIReturnsLatestTagOnSuccess(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &githubCLIRunner{latestOutput: []byte(`{"tag_name":"v9.9.9"}`)}}
	tag, err := u.latestTagFromGitHubCLI(context.Background(), source)
	if err != nil || tag != "v9.9.9" {
		t.Fatalf("tag=%q err=%v", tag, err)
	}
}

func TestLatestTagFromGitHubCLIFallsBackToListWhenLatestFails(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	list := `[{"tag_name":"v1.0.0-rc.1","prerelease":true,"published_at":"2026-01-01T00:00:00Z"}]`
	u := Updater{Runner: &githubCLIRunner{latestErr: errors.New("latest failed"), listOutput: []byte(list)}}
	tag, err := u.latestTagFromGitHubCLI(context.Background(), source)
	if err != nil || tag != "v1.0.0-rc.1" {
		t.Fatalf("tag=%q err=%v", tag, err)
	}
}

func TestLatestTagFromGitHubCLIPropagatesListFailure(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &githubCLIRunner{latestErr: errors.New("latest failed"), listErr: errors.New("list failed")}}
	if _, err := u.latestTagFromGitHubCLI(context.Background(), source); err == nil || !strings.Contains(err.Error(), "list failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitHubCLIRejectsMalformedListJSON(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &githubCLIRunner{latestErr: errors.New("latest failed"), listOutput: []byte("not-json")}}
	if _, err := u.latestTagFromGitHubCLI(context.Background(), source); err == nil || !strings.Contains(err.Error(), "parse GitHub release metadata through gh") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestTagFromGitHubCLIReturnsOriginalErrorWithoutPublishedPrerelease(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	original := errors.New("latest failed")
	u := Updater{Runner: &githubCLIRunner{latestErr: original, listOutput: []byte(`[]`)}}
	_, err := u.latestTagFromGitHubCLI(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "latest failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsWithCLIRejectsDisallowedSource(t *testing.T) {
	u := Updater{}
	source := ReleaseSource{Origin: "https://example.test", Repository: "o/r"}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil {
		t.Fatal("downloadGitHubReleaseAssetsWithCLI accepted a disallowed source")
	}
}

func TestDownloadGitHubReleaseAssetsWithCLIRejectsInvalidAssetName(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &fakeCommandRunner{}}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", t.TempDir(), "nested/asset"); err == nil {
		t.Fatal("downloadGitHubReleaseAssetsWithCLI accepted a nested asset name")
	}
}

func TestDownloadGitHubReleaseAssetsWithCLIPropagatesRunFailure(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &fakeCommandRunner{fail: map[string]error{"gh": errors.New("boom")}}}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsWithCLIRejectsMissingWrittenFile(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &fakeCommandRunner{}}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "did not write") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsWithCLIRejectsWrittenDirectory(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "asset.tar.gz"), 0o700); err != nil {
		t.Fatal(err)
	}
	u := Updater{Runner: &fakeCommandRunner{}}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", directory, "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "did not write") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsWithCLISucceedsWhenAssetIsWritten(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	directory := t.TempDir()
	runner := &writingCommandRunner{directory: directory}
	u := Updater{Runner: runner}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", directory, "asset.tar.gz"); err != nil {
		t.Fatal(err)
	}
}

type writingCommandRunner struct {
	directory string
}

func (r *writingCommandRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if name == "gh" {
		return nil, os.WriteFile(filepath.Join(r.directory, "asset.tar.gz"), []byte("payload"), 0o600)
	}
	return nil, errors.New("unexpected command")
}
