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

func TestUpdateUsesPublishedGitHubPrereleaseWhenNoStableReleaseExists(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0-rc.1_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0-rc.1_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/example-owner/aigw-cli/releases/latest":
			http.NotFound(w, request)
		case "/repos/example-owner/aigw-cli/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `[{"tag_name":"v0.2.0-rc.1","prerelease":true,"published_at":"2026-07-15T00:00:00Z"},{"tag_name":"v0.3.0-rc.1","prerelease":true,"draft":true,"published_at":"2026-07-15T00:00:00Z"}]`)
		case "/repos/example-owner/aigw-cli/releases/tags/v0.2.0-rc.1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"tag_name":"v0.2.0-rc.1","prerelease":true,"published_at":"2026-07-15T00:00:00Z","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
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
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &missingGlabRunner{}, HTTPClient: server.Client()}
	message, err := u.Update(context.Background(), "0.1.0-rc.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "v0.2.0-rc.1") {
		t.Fatalf("message = %q", message)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "candidate-binary" {
		t.Fatalf("binary=%q error=%v", got, err)
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
	gitlab := &fakeRunner{archive: archive, checksum: checksum, tag: "Warning: Multiple config files found.\nUsing: /tmp/glab/configuration.yml\nv0.2.0\n"}
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

func TestUpdateUsesAuthenticatedGHCLIForPrivatePublishedPrerelease(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0-rc.1_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0-rc.1_darwin_arm64.tar.gz"
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &githubKeyringRunner{archive: archive, checksum: checksum}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "api.github.com" {
			return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader(`{"message":"Not Found"}`)), Header: make(http.Header), Request: request}, nil
		}
		return nil, fmt.Errorf("unexpected HTTP request: %s", request.URL)
	})}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: binary,
		Runner:     runner,
		HTTPClient: client,
		GitLab:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject},
		GitHub:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitHub, Origin: "https://github.com", Repository: "example-owner/aigw-cli"},
	}
	message, err := u.Update(context.Background(), "0.1.0-rc.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "v0.2.0-rc.1") || !strings.Contains(message, "github") {
		t.Fatalf("message = %q", message)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "candidate-binary" {
		t.Fatalf("binary=%q error=%v", got, err)
	}
	if !runner.called("gh", "api", "repos/example-owner/aigw-cli/releases/latest") ||
		!runner.called("gh", "api", "repos/example-owner/aigw-cli/releases?per_page=100") ||
		!runner.called("gh", "release", "download") {
		t.Fatalf("GH CLI fallback calls = %v", runner.calls)
	}
}

func TestUpdateDoesNotUseGHCLIForCustomGitHubOrigin(t *testing.T) {
	runner := &githubKeyringRunner{}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader(`{"message":"Not Found"}`)), Header: make(http.Header), Request: request}, nil
	})}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
		HTTPClient: client,
		GitHub:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitHub, Origin: "https://github.example.test", Repository: "example-owner/aigw-cli"},
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if call[0] == "gh" {
			t.Fatalf("custom origin unexpectedly invoked gh: %v", runner.calls)
		}
	}
}

func TestUpdateDoesNotUseGHCLIForNonNotFoundGitHubFailure(t *testing.T) {
	runner := &githubKeyringRunner{}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error", Body: io.NopCloser(strings.NewReader(`{"message":"Internal Server Error"}`)), Header: make(http.Header), Request: request}, nil
	})}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
		HTTPClient: client,
		GitHub:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitHub, Origin: "https://github.com", Repository: "example-owner/aigw-cli"},
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if call[0] == "gh" {
			t.Fatalf("non-404 failure unexpectedly invoked gh: %v", runner.calls)
		}
	}
}

func TestUpdateRequiresAnExplicitReleaseSource(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	runner := &fakeRunner{}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "release source is not configured") {
		t.Fatalf("Update() error = %v, want explicit release-source configuration error", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("Update() invoked a release client without a release source: %v", runner.calls)
	}
}

func TestUpdateRejectsPartialGitLabTupleBeforeContactingAForge(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
	runner := &fakeRunner{}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(t.TempDir(), "aigw"), Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "source is incomplete") {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("partial tuple invoked a release client: %v", runner.calls)
	}
}

func TestUpdateRejectsInvalidGitLabTupleBeforeContactingAForge(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://legacy.example.test/path")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", testReleaseProject)
	runner := &fakeRunner{}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(t.TempDir(), "aigw"), Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "release origin") {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("mixed tuple invoked a release client: %v", runner.calls)
	}
}

func TestUpdateRefusesOlderStableReleaseWithoutReplacingBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.1.9_darwin_arm64/aigw", []byte("old-release-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.1.9_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.1.9"}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "older") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "current-binary" {
		t.Fatalf("binary was replaced by older release: %q", got)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("older release should fail before downloading assets: %v", runner.calls)
	}
}

func TestUpdateRefusesOlderPrereleaseWithoutReplacingBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0-rc.1_darwin_arm64/aigw", []byte("old-release-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0-rc.1_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.2.0-rc.1"}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.2.0-rc.2")
	if err == nil || !strings.Contains(err.Error(), "older") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "current-binary" {
		t.Fatalf("binary was replaced by older prerelease: %q", got)
	}
}

func TestUpdateAcceptsStableReleaseAfterPrerelease(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("stable-release-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("prerelease-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.2.0"}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.2.0-rc.2"); err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "stable-release-binary" {
		t.Fatalf("binary = %q", got)
	}
}

func TestUpdateAcceptsNumericallyNewerPrerelease(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0-rc.10_darwin_arm64/aigw", []byte("newer-prerelease-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0-rc.10_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("older-prerelease-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.2.0-rc.10"}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.2.0-rc.9"); err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "newer-prerelease-binary" {
		t.Fatalf("binary = %q", got)
	}
}

func TestUpdateRejectsInvalidCurrentVersionBeforeDownloading(t *testing.T) {
	runner := &fakeRunner{tag: "v0.2.0"}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
	}
	_, err := u.Update(context.Background(), "development-build")
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("error = %v", err)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("invalid current version should fail before downloading assets: %v", runner.calls)
	}
}

func TestUpdateRejectsMalformedReleaseVersionBeforeDownloading(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{tag: "release-candidate"}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("error = %v", err)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("malformed release tag should fail before downloading assets: %v", runner.calls)
	}
}

func TestUpdateRejectsOverflowingReleaseVersionBeforeDownloading(t *testing.T) {
	runner := &fakeRunner{tag: "v18446744073709551616.0.0"}
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     runner,
	}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("error = %v", err)
	}
	if runner.called("glab", "release", "download") {
		t.Fatalf("overflowing release tag should fail before downloading assets: %v", runner.calls)
	}
}

func TestCurrentUsesBuildTimeGitHubSource(t *testing.T) {
	previousOrigin, previousRepository := upgrade.BuildGitHubReleaseOrigin, upgrade.BuildGitHubReleaseRepository
	t.Cleanup(func() {
		upgrade.BuildGitHubReleaseOrigin, upgrade.BuildGitHubReleaseRepository = previousOrigin, previousRepository
	})
	upgrade.BuildGitHubReleaseOrigin = "https://github.com"
	upgrade.BuildGitHubReleaseRepository = "example-owner/aigw-cli"
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "")
	updater := upgrade.Current(filepath.Join(t.TempDir(), "aigw"))
	if got := updater.GitHub; got != (upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitHub, Origin: "https://github.com", Repository: "example-owner/aigw-cli"}) {
		t.Fatalf("GitHub source = %#v", got)
	}
}

func TestCurrentUsesBuildTimeGitLabSource(t *testing.T) {
	previousOrigin, previousRepository := upgrade.BuildGitLabReleaseOrigin, upgrade.BuildGitLabReleaseRepository
	t.Cleanup(func() {
		upgrade.BuildGitLabReleaseOrigin, upgrade.BuildGitLabReleaseRepository = previousOrigin, previousRepository
	})
	upgrade.BuildGitLabReleaseOrigin = "https://gitlab.example.test"
	upgrade.BuildGitLabReleaseRepository = testReleaseProject
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	updater := upgrade.Current(filepath.Join(t.TempDir(), "aigw"))
	if got := updater.GitLab; got != (upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: testReleaseProject}) {
		t.Fatalf("GitLab source = %#v", got)
	}
}

func TestExplicitReleaseSourceEnvironmentOverridesBuildMetadata(t *testing.T) {
	previousOrigin, previousRepository := upgrade.BuildGitLabReleaseOrigin, upgrade.BuildGitLabReleaseRepository
	t.Cleanup(func() {
		upgrade.BuildGitLabReleaseOrigin, upgrade.BuildGitLabReleaseRepository = previousOrigin, previousRepository
	})
	upgrade.BuildGitLabReleaseOrigin = "https://embedded.example.test"
	upgrade.BuildGitLabReleaseRepository = "embedded/project"
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://override.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", testReleaseProject)
	runner := &fakeRunner{}
	u := upgrade.Current(filepath.Join(t.TempDir(), "aigw"))
	u.Runner = runner
	if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if containsSequence(call, "-R", testReleaseProject) {
			return
		}
	}
	t.Fatalf("release project override was not passed to glab: %v", runner.calls)
}
