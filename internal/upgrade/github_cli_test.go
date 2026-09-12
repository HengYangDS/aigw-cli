package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type githubCLIRunner struct {
	latestOutput []byte
	latestErr    error
	listOutput   []byte
	listErr      error
}

func TestLatestGitHubReleaseWithCLIPropagatesRunError(t *testing.T) {
	u := Updater{Runner: &githubCLIRunner{latestErr: errors.New("boom")}}
	if _, err := u.latestGitHubReleaseWithCLI(context.Background(), ReleaseSource{Repository: "o/r"}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestGitHubReleaseWithCLIRejectsMalformedJSON(t *testing.T) {
	u := Updater{Runner: &githubCLIRunner{latestOutput: []byte("not-json")}}
	if _, err := u.latestGitHubReleaseWithCLI(context.Background(), ReleaseSource{Repository: "o/r"}); err == nil || !strings.Contains(err.Error(), "parse GitHub release metadata through gh") {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestGitHubReleaseWithCLIReturnsDecodedRelease(t *testing.T) {
	u := Updater{Runner: &githubCLIRunner{latestOutput: []byte(`{"tag_name":"v1.2.3"}`)}}
	release, err := u.latestGitHubReleaseWithCLI(context.Background(), ReleaseSource{Repository: "o/r"})
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
	t.Setenv("GH_HOST", "unrelated.example.test")
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	runner := &recordingRunner{output: []byte(`{"tag_name":"v9.9.9"}`)}
	u := Updater{Runner: runner}
	tag, err := u.latestTagFromGitHubCLI(context.Background(), source)
	if err != nil || tag != "v9.9.9" {
		t.Fatalf("tag=%q err=%v", tag, err)
	}
	if len(runner.plans) != 1 || !slices.Contains(runner.plans[0].Env, "GH_HOST=github.com") {
		t.Fatal("GitHub CLI did not bind the selected source")
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
	u := Updater{Runner: &recordingRunner{}}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", t.TempDir(), "nested/asset"); err == nil {
		t.Fatal("downloadGitHubReleaseAssetsWithCLI accepted a nested asset name")
	}
}

func TestDownloadGitHubReleaseAssetsWithCLIPropagatesRunFailure(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &recordingRunner{err: errors.New("boom")}}
	if err := u.downloadGitHubReleaseAssetsWithCLI(context.Background(), source, "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadGitHubReleaseAssetsWithCLIRejectsMissingWrittenFile(t *testing.T) {
	source := ReleaseSource{Origin: "https://github.com", Repository: "o/r"}
	u := Updater{Runner: &recordingRunner{}}
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
	u := Updater{Runner: &recordingRunner{}}
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
