package acceptance_test

import (
	"aigw-cli/internal/upgrade"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUpdateRequiresAnExplicitReleaseSource(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	runner := &releaseRunner{}
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
	runner := &releaseRunner{}
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
	runner := &releaseRunner{}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(t.TempDir(), "aigw"), Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "release origin") {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("mixed tuple invoked a release client: %v", runner.calls)
	}
}

func TestUpdateRejectsAmbiguousReleaseCoordinatesBeforeContact(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	for _, provider := range []string{"GITLAB", "GITHUB"} {
		for _, test := range []struct {
			name, suffix, repository string
		}{
			{name: "empty query", suffix: "?", repository: "group/project"},
			{name: "empty fragment", suffix: "#", repository: "group/project"},
			{name: "non-root path", suffix: "///", repository: "group/project"},
			{name: "missing namespace", repository: "project"},
			{name: "escaped traversal", repository: "group/%2e%2e"},
			{name: "escaped separator", repository: "group/project%2Fother"},
			{name: "whitespace", repository: "group/project name"},
		} {
			t.Run(provider+"/"+test.name, func(t *testing.T) {
				requestsBefore := requests.Load()
				for _, peer := range []string{"GITLAB", "GITHUB"} {
					t.Setenv("AIGW_"+peer+"_RELEASE_ORIGIN", "")
					t.Setenv("AIGW_"+peer+"_RELEASE_REPOSITORY", "")
				}
				t.Setenv("AIGW_"+provider+"_RELEASE_ORIGIN", server.URL+test.suffix)
				t.Setenv("AIGW_"+provider+"_RELEASE_REPOSITORY", test.repository)
				runner := &releaseRunner{}
				updater := upgrade.Updater{Runner: runner, HTTPClient: server.Client()}
				if _, err := updater.Update(t.Context(), "1.0.0"); err == nil {
					t.Fatal("ambiguous source was admitted")
				}
				if called := requests.Load() - requestsBefore; called != 0 || len(runner.calls) != 0 {
					t.Fatalf("invalid source reached a peer: HTTP=%d CLI=%v", called, runner.calls)
				}
			})
		}
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
	runner := &releaseRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.1.9"}
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
	if calledCommand(runner.calls, "glab", "release", "download") {
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
	runner := &releaseRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: "v0.2.0-rc.1"}
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

func TestUpdateAcceptsNewerSemanticVersion(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		tag            string
		binary         string
	}{
		{name: "stable after prerelease", currentVersion: "0.2.0-rc.2", tag: "v0.2.0", binary: "stable-release-binary"},
		{name: "numerically newer prerelease", currentVersion: "0.2.0-rc.9", tag: "v0.2.0-rc.10", binary: "newer-prerelease-binary"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version := strings.TrimPrefix(test.tag, "v")
			archiveName := fmt.Sprintf("aigw_%s_darwin_arm64.tar.gz", version)
			archive := tarGz(t, fmt.Sprintf("aigw_%s_darwin_arm64/aigw", version), []byte(test.binary))
			sum := sha256.Sum256(archive)
			binary := filepath.Join(t.TempDir(), "aigw")
			if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			runner := &releaseRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, archiveName), tag: test.tag, version: version}
			u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
			if _, err := u.Update(context.Background(), test.currentVersion); err != nil {
				t.Fatal(err)
			}
			got, readErr := os.ReadFile(binary)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != test.binary {
				t.Fatalf("binary = %q, want %q", got, test.binary)
			}
		})
	}
}

func TestUpdateRejectsInvalidVersionsBeforeDownloading(t *testing.T) {
	for _, test := range []struct {
		name, current, tag string
	}{
		{"invalid current", "development-build", "v0.2.0"},
		{"malformed release", "0.2.0", "release-candidate"},
		{"overflowing release", "0.2.0", "v18446744073709551616.0.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "aigw")
			if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			runner := &releaseRunner{tag: test.tag}
			u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
			_, err := u.Update(context.Background(), test.current)
			if err == nil || !strings.Contains(err.Error(), "invalid release version") {
				t.Fatalf("error = %v", err)
			}
			if calledCommand(runner.calls, "glab", "release", "download") {
				t.Fatalf("invalid version triggered an asset download: %v", runner.calls)
			}
			if got, err := os.ReadFile(binary); err != nil || string(got) != "current-binary" {
				t.Fatalf("invalid version changed installed bytes: %q, %v", got, err)
			}
		})
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
	runner := &releaseRunner{}
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
