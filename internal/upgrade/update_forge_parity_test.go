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
	"strings"
	"testing"

	"aigw-cli/internal/upgrade"
)

func TestUpdateRejectsPeerIntegrityFailureBeforeReplacingBinary(t *testing.T) {
	gitlabArchive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("gitlab-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	githubRequests := 0
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		githubRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v0.2.0"}`)
	}))
	defer github.Close()
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", github.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	runner := &fakeRunner{archive: gitlabArchive, checksum: strings.Repeat("0", 64) + "  " + archiveName + "\n"}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     runner,
		HTTPClient: github.Client(),
		GitLab:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	if githubRequests != 1 {
		t.Fatalf("GitHub metadata requests=%d, want 1", githubRequests)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary replaced after GitLab checksum failure: %q", got)
	}
}

func TestUpdateRejectsReachablePeerAuthorizationFailure(t *testing.T) {
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	githubRequests := 0
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		githubRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v0.2.0"}`)
	}))
	defer github.Close()
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", github.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	runner := &fakeRunner{archive: []byte("unused"), checksum: "unused"}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Host, "gitlab.example.test") {
			return &http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", Body: io.NopCloser(strings.NewReader("denied")), Header: make(http.Header), Request: request}, nil
		}
		return github.Client().Transport.RoundTrip(request)
	})}
	u := upgrade.Updater{
		GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}, HTTPClient: client,
		GitLab: upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
	}
	t.Setenv("GITLAB_TOKEN", "test-token")
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("error = %v", err)
	}
	if githubRequests != 0 {
		t.Fatalf("GitHub must not be contacted after a terminal GitLab authorization failure: %d", githubRequests)
	}
	if runner.downloaded(archiveName) {
		t.Fatalf("GitLab archive downloaded after failed authorization: %v", runner.calls)
	}
}

func TestUpdateUsesReachablePeerWhenOtherPeerIsUnavailable(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("github-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
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
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sum, archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer github.Close()
	githubURL = github.URL
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", testReleaseProject)
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", github.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	t.Setenv("GITLAB_TOKEN", "")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Host, "gitlab.example.test") {
			return nil, fmt.Errorf("simulated GitLab transport outage")
		}
		return github.Client().Transport.RoundTrip(request)
	})}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     &missingGlabRunner{},
		HTTPClient: client,
	}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "github-binary" || !strings.Contains(message, "github") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
}

func TestUpdateUsesGitHubPeerAfterGitLabAPITransportFailure(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("github-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest", "/repos/example-owner/aigw-cli/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				archiveName, serverURL+"/downloads/"+archiveName, serverURL+"/downloads/checksums.txt")
		case "/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sum, archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", server.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	t.Setenv("GITLAB_TOKEN", "test-token")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Host, "gitlab.example.test") {
			return nil, fmt.Errorf("simulated GitLab transport outage")
		}
		return server.Client().Transport.RoundTrip(request)
	})}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     &missingGlabRunner{},
		HTTPClient: client,
		GitLab:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
	}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "github-binary" || !strings.Contains(message, "github") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
}

func TestUpdateUsesGitHubPeerWhenGitLabIsUnavailable(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("github-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	requests := []string{}
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Path)
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest", "/repos/example-owner/aigw-cli/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				archiveName, serverURL+"/downloads/"+archiveName, serverURL+"/downloads/checksums.txt")
		case "/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sum, archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	githubHost := strings.TrimPrefix(server.URL, "http://")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", testReleaseProject)
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "http://"+githubHost)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     &missingGlabRunner{},
		HTTPClient: server.Client(),
	}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "github-binary" || !strings.Contains(message, "github") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
	if len(requests) != 4 || requests[0] != "/repos/example-owner/aigw-cli/releases/latest" || requests[1] != "/repos/example-owner/aigw-cli/releases/tags/v0.2.0" {
		t.Fatalf("GitHub peer requests = %v", requests)
	}
}

func TestUpdateVerifiesMatchingPeerReleasesBeforeInstalling(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("same-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	requests := 0
	var githubURL string
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest", "/repos/example-owner/aigw-cli/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				archiveName, githubURL+"/downloads/"+archiveName, githubURL+"/downloads/checksums.txt")
		case "/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sum, archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer github.Close()
	githubURL = github.URL
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", github.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n", sum, archiveName)}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     runner,
		HTTPClient: github.Client(),
		GitLab:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
	}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 4 {
		t.Fatalf("GitHub requests = %d, want metadata plus both assets", requests)
	}
	if !strings.Contains(message, "gitlab and github") {
		t.Fatalf("message = %q", message)
	}
}

func TestUpdateRejectsPeerTagDisagreementBeforeDownloading(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("candidate"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	requests := 0
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v0.3.0"}`)
	}))
	defer github.Close()
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", github.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     runner,
		HTTPClient: github.Client(),
		GitLab:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "disagree on latest tag") {
		t.Fatalf("error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("GitHub metadata requests = %d, want 1", requests)
	}
	if runner.downloaded(archiveName) {
		t.Fatalf("GitLab assets were downloaded before peer tag agreement: %v", runner.calls)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "old-binary" {
		t.Fatalf("binary=%q error=%v", got, err)
	}
}

func TestUpdateDiscardsUnavailablePeerWorkspaceBeforeUsingReachablePeer(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("same-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
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
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sum, archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer github.Close()
	githubURL = github.URL
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", github.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	missingRunner := &fakeRunner{}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     missingRunner,
		HTTPClient: github.Client(),
		GitLab:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
	}

	updateTempRoot := t.TempDir()
	t.Setenv("TMPDIR", updateTempRoot)
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "github") || strings.Contains(message, "gitlab and github") {
		t.Fatalf("message = %q", message)
	}
	after, err := filepath.Glob(filepath.Join(updateTempRoot, "aigw-update-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("unavailable peer left update workspace: %v", after)
	}
}

func TestUpdateRejectsPeerAssetDisagreementBeforeReplacingBinary(t *testing.T) {
	gitlabArchive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("gitlab-binary"))
	githubArchive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("github-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	var githubURL string
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest", "/repos/example-owner/aigw-cli/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				archiveName, githubURL+"/downloads/"+archiveName, githubURL+"/downloads/checksums.txt")
		case "/downloads/" + archiveName:
			_, _ = w.Write(githubArchive)
		case "/downloads/checksums.txt":
			_, _ = w.Write([]byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(githubArchive), archiveName)))
		default:
			http.NotFound(w, request)
		}
	}))
	defer github.Close()
	githubURL = github.URL
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", github.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	runner := &fakeRunner{archive: gitlabArchive, checksum: fmt.Sprintf("%x  %s\n", sha256.Sum256(gitlabArchive), archiveName)}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     runner,
		HTTPClient: github.Client(),
		GitLab:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "asset bytes") {
		t.Fatalf("error = %v", err)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "old-binary" {
		t.Fatalf("binary=%q error=%v", got, err)
	}
}
