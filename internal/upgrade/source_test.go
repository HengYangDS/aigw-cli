package upgrade

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReleaseSourceSnapshotBindsGitLabProcess(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://selected.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "selected/project")
	runner := &recordingRunner{output: []byte("v1.0.0\n")}
	u := Updater{Runner: runner}
	source := u.forgeSources()[0]
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://changed.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "changed/project")

	tag, unavailableFlag, err := u.latestTagFromSource(t.Context(), source)
	if err != nil || unavailableFlag || tag != "v1.0.0" {
		t.Fatalf("release = %q, unavailable = %v, error = %v", tag, unavailableFlag, err)
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "asset.tar.gz"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if unavailableFlag, err := u.downloadReleaseAssetsFromExactSource(t.Context(), source, tag, directory, "asset.tar.gz"); err != nil || unavailableFlag {
		t.Fatalf("download unavailable = %v, error = %v", unavailableFlag, err)
	}
	if len(runner.plans) != 2 {
		t.Fatalf("process plans = %d, want metadata and asset download", len(runner.plans))
	}
	for _, plan := range runner.plans {
		if !slices.Contains(plan.Env, "GITLAB_HOST="+source.Origin) {
			t.Error("process host differs from selected release source")
		}
		if !strings.Contains(strings.Join(plan.Args, " "), "-R "+source.Repository) {
			t.Errorf("process repository differs from selected release source: %v", plan.Args)
		}
	}
}

func TestReleaseSourceSnapshotBindsGitLabHTTPFallback(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://selected.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "selected/project")
	t.Setenv("GITLAB_TOKEN", "test-token")
	var requests []string
	u := Updater{
		Runner: &recordingRunner{err: exec.ErrNotFound},
		HTTPClient: &http.Client{Transport: githubRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.URL.String())
			if request.URL.Host != "selected.example.test" || request.Header.Get("Private-Token") != "test-token" {
				t.Errorf("request does not use selected authenticated source: %s", request.URL)
			}
			body := "payload"
			if strings.HasSuffix(request.URL.Path, "/permalink/latest") {
				body = `{"tag_name":"v1.0.0"}`
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
	}
	source := u.forgeSources()[0]
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://changed.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "changed/project")
	tag, unavailableFlag, err := u.latestTagFromSource(t.Context(), source)
	if err != nil || unavailableFlag || tag != "v1.0.0" {
		t.Fatalf("release = %q, unavailable = %v, error = %v", tag, unavailableFlag, err)
	}
	directory := t.TempDir()
	if unavailableFlag, err := u.downloadReleaseAssetsFromExactSource(t.Context(), source, tag, directory, "asset.tar.gz"); err != nil || unavailableFlag {
		t.Fatalf("download unavailable = %v, error = %v", unavailableFlag, err)
	}
	want := []string{
		source.Origin + "/api/v4/projects/selected%2Fproject/releases/permalink/latest",
		source.Origin + "/selected/project/-/releases/v1.0.0/downloads/asset.tar.gz",
	}
	if !slices.Equal(requests, want) {
		t.Errorf("requests = %v, want %v", requests, want)
	}
	if payload, err := os.ReadFile(filepath.Join(directory, "asset.tar.gz")); err != nil || string(payload) != "payload" {
		t.Fatalf("asset = %q, error = %v", payload, err)
	}
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

func TestValidateReleaseSourceRejectsUnsupportedProvider(t *testing.T) {
	source := ReleaseSource{Provider: "bogus", Origin: "https://example.test", Repository: "o/r"}
	if err := validateReleaseSource(source); err == nil || !strings.Contains(err.Error(), "unsupported release provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateReleaseSourceRejectsGitHubOverPlainHTTPForRealHost(t *testing.T) {
	source := ReleaseSource{Provider: ReleaseProviderGitHub, Origin: "http://github.example.com", Repository: "o/r"}
	if err := validateReleaseSource(source); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateReleaseSourceRejectsGitLabOverPlainHTTPForRealHost(t *testing.T) {
	source := ReleaseSource{Provider: ReleaseProviderGitLab, Origin: "http://gitlab.example.com", Repository: "group/project"}
	if err := validateReleaseSource(source); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateReleaseSourceAllowsPrivateGitLabOverPlainHTTP(t *testing.T) {
	for _, origin := range []string{
		"http://192.168.64.101:18086",
		"http://10.0.0.8",
		"http://[fd00::8]:8080",
	} {
		source := ReleaseSource{Provider: ReleaseProviderGitLab, Origin: origin, Repository: "group/project"}
		if err := validateReleaseSource(source); err != nil {
			t.Fatalf("origin=%q error = %v", origin, err)
		}
	}
}

func TestValidateReleaseSourceAllowsGitHubOverPlainHTTPForTestHosts(t *testing.T) {
	cases := []string{"http://example.test", "http://localhost:8080", "http://127.0.0.1:8080"}
	for _, origin := range cases {
		source := ReleaseSource{Provider: ReleaseProviderGitHub, Origin: origin, Repository: "o/r"}
		if err := validateReleaseSource(source); err != nil {
			t.Fatalf("origin=%q error = %v", origin, err)
		}
	}
}

func TestValidateReleaseSourceRejectsInvalidRepositoryPath(t *testing.T) {
	cases := []string{"/o/r", "o/r/", "o/r?x=1", "o/r#frag", "o\r\n/r", "project", "o/r with space", "o/r\tname", "o/%2e%2e", "o/r%2Fother"}
	for _, repository := range cases {
		source := ReleaseSource{Provider: ReleaseProviderGitLab, Origin: "https://example.test", Repository: repository}
		if err := validateReleaseSource(source); err == nil {
			t.Fatalf("repository=%q: accepted an invalid path", repository)
		}
	}
}

func TestValidateReleaseSourceRequiresOneAuthority(t *testing.T) {
	for _, origin := range []string{
		"https://:443", "https://example.test?", "https://example.test#", "https://example.test///",
	} {
		t.Run(origin, func(t *testing.T) {
			source := ReleaseSource{Provider: ReleaseProviderGitLab, Origin: origin, Repository: "group/project"}
			if err := validateReleaseSource(source); err == nil {
				t.Fatalf("non-authority origin %q was admitted", origin)
			}
		})
	}
}

func TestValidateReleaseSourceRejectsGitHubRepositoryWithWrongPartCount(t *testing.T) {
	source := ReleaseSource{Provider: ReleaseProviderGitHub, Origin: "https://api.github.com", Repository: "only-one-part"}
	if err := validateReleaseSource(source); err == nil || !strings.Contains(err.Error(), "owner/repository path") {
		t.Fatalf("error = %v", err)
	}
	source.Repository = "a/b/c"
	if err := validateReleaseSource(source); err == nil || !strings.Contains(err.Error(), "owner/repository path") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateReleaseSourceRejectsRepositoryPartsWithDotDotOrBackslash(t *testing.T) {
	cases := []string{"group/..", "group/.", "group\\name/project", "group/"}
	for _, repository := range cases {
		source := ReleaseSource{Provider: ReleaseProviderGitLab, Origin: "https://example.test", Repository: repository}
		if err := validateReleaseSource(source); err == nil {
			t.Fatalf("repository=%q: accepted an invalid segment", repository)
		}
	}
}

func TestValidateReleaseSourceAcceptsNestedGitLabNamespace(t *testing.T) {
	for _, source := range []ReleaseSource{
		{Provider: ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: "group/subgroup/project"},
		{Provider: ReleaseProviderGitLab, Origin: "https://[fd00::8]:8443/", Repository: "group_name/project.name-1"},
		{Provider: ReleaseProviderGitHub, Origin: "https://github.example.test/", Repository: "owner_name/project.name-1"},
	} {
		if err := validateReleaseSource(source); err != nil {
			t.Fatalf("valid source %#v: %v", source, err)
		}
	}
}

func TestUnwrapReturnsWrappedError(t *testing.T) {
	inner := errors.New("boom")
	wrapped := sourceUnavailableError{err: inner}
	if !errors.Is(wrapped.Unwrap(), inner) {
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
