package upgrade

import (
	"aigw-cli/internal/process"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadReleaseAssetsWithGlabAPISkipsLinksWithEmptyURL(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":""},{"name":"asset.tar.gz","url":"https://example.test/downloads/asset.tar.gz"}]}}`
	directory := t.TempDir()
	runner := &recordingFileRunner{output: []byte(metadata), content: []byte("downloaded")}
	u := Updater{Runner: runner}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", directory, "asset.tar.gz"); err != nil {
		t.Fatal(err)
	}
}

func TestGitLabMetadataUsesExplicitProcessPlan(t *testing.T) {
	t.Setenv("GITLAB_HOST", "https://unrelated.example.test")
	runner := &recordingRunner{output: []byte("v1.0.0\n")}
	u := Updater{Runner: runner, GitLab: ReleaseSource{Origin: "https://gitlab.example.test", Repository: "group/project"}}
	if _, err := u.runGlab(t.Context(), "api", "projects"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 {
		t.Fatalf("captured plans = %d, want one explicit invocation", len(runner.plans))
	}
	plan := runner.plans[0]
	if plan.Executable != "glab" || strings.Join(plan.Args, " ") != "api projects" {
		t.Fatalf("unexpected process plan: %#v", plan)
	}
	if !slices.Contains(plan.Env, "GITLAB_HOST=https://gitlab.example.test") {
		t.Fatal("configured GitLab host was not bound to the process plan")
	}
}

func TestGitLabAssetUsesExplicitProcessPlan(t *testing.T) {
	runner := &recordingFileRunner{content: []byte("payload")}
	u := Updater{Runner: runner, GitLab: ReleaseSource{Origin: "https://gitlab.example.test", Repository: "group/project"}}
	destination := filepath.Join(t.TempDir(), "asset")
	if err := u.runGlabToFile(t.Context(), destination, "api", "asset"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || len(runner.destinations) != 1 || runner.destinations[0] != destination {
		t.Fatalf("file invocation = %#v", runner)
	}
	plan := runner.plans[0]
	if plan.Executable != "glab" || strings.Join(plan.Args, " ") != "api asset" || !slices.Contains(plan.Env, "GITLAB_HOST=https://gitlab.example.test") {
		t.Fatalf("asset process plan = %#v", plan)
	}
	if content, err := os.ReadFile(destination); err != nil || string(content) != "payload" {
		t.Fatalf("asset output = %q, %v", content, err)
	}
}

func TestGitLabCLIUsesSelectedOriginDespiteAmbientConfiguration(t *testing.T) {
	var selectedRequests, ambientRequests atomic.Int32
	selected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		selectedRequests.Add(1)
		if request.URL.Path != "/api/v4/version" || request.Header.Get("Private-Token") != "synthetic-loopback-only" {
			t.Errorf("selected origin received unexpected request: %s", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"source":"selected"}`)
	}))
	t.Cleanup(selected.Close)
	ambient := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ambientRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"source":"ambient"}`)
	}))
	t.Cleanup(ambient.Close)
	root := t.TempDir()
	t.Chdir(root)
	config := fmt.Sprintf("host: %q\nno_prompt: true\nhosts:\n  %q:\n    api_host: %q\n    api_protocol: https\n    subfolder: legacy\n    use_keyring: false\n", ambient.Listener.Addr().String(), selected.Listener.Addr().String(), ambient.Listener.Addr().String())
	if err := os.WriteFile(filepath.Join(root, "config.yml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"HOME": root, "USERPROFILE": root, "GLAB_CONFIG_DIR": root,
		"GITLAB_TOKEN": "synthetic-loopback-only", "USE_KEYRING": "false",
		"GITLAB_HOST": ambient.URL, "GITLAB_URI": ambient.URL, "GL_HOST": ambient.URL,
		"GITLAB_API_HOST": ambient.Listener.Addr().String(), "API_PROTOCOL": "http", "GITLAB_SUBFOLDER": "legacy",
		"GLAB_NO_PROMPT": "true", "GLAB_CHECK_UPDATE": "false", "GLAB_SHOW_WHATS_NEW": "false", "GLAB_SEND_TELEMETRY": "false",
	} {
		t.Setenv(key, value)
	}
	u := Updater{Runner: process.Runner{}, GitLab: ReleaseSource{Origin: selected.URL, Repository: "group/project"}}
	output, err := u.runGlab(t.Context(), "api", "version")
	if err != nil || strings.TrimSpace(string(output)) != `{"source":"selected"}` {
		t.Fatalf("selected CLI metadata = %q, %v", output, err)
	}
	destination := filepath.Join(root, "asset")
	if err := u.runGlabToFile(t.Context(), destination, "api", "version"); err != nil {
		t.Fatal(err)
	}
	if output, err := os.ReadFile(destination); err != nil || strings.TrimSpace(string(output)) != `{"source":"selected"}` {
		t.Fatalf("selected CLI asset = %q, %v", output, err)
	}
	if selectedRequests.Load() != 2 || ambientRequests.Load() != 0 {
		t.Fatalf("source routing: selected=%d ambient=%d", selectedRequests.Load(), ambientRequests.Load())
	}
}

func TestRunGlabToFileRejectsRunnerWithoutFileSupport(t *testing.T) {
	u := Updater{Runner: &recordingRunner{}}
	if err := u.runGlabToFile(context.Background(), "/tmp/dest", "api", "asset"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRequiresFileCapableRunner(t *testing.T) {
	u := Updater{Runner: &recordingRunner{}}
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
	u := Updater{Runner: &recordingFileRunner{err: errors.New("boom")}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsMalformedMetadata(t *testing.T) {
	u := Updater{Runner: &recordingFileRunner{output: []byte("not-json")}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "parse release metadata") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsInvalidAssetName(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/asset.tar.gz"}]}}`
	u := Updater{Runner: &recordingFileRunner{output: []byte(metadata)}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "nested/asset"); err == nil {
		t.Fatal("downloadReleaseAssetsWithGlabAPI accepted a nested asset name")
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsMissingAssetURL(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"other.tar.gz","url":"https://example.test/other.tar.gz"}]}}`
	u := Updater{Runner: &recordingFileRunner{output: []byte(metadata)}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "does not include") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIPropagatesDownloadFailure(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/asset.tar.gz"}]}}`
	u := Updater{Runner: &recordingFileRunner{output: []byte(metadata), fileErr: errors.New("download failed")}}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", t.TempDir(), "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIRejectsMissingWrittenFile(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/asset.tar.gz"}]}}`
	directory := t.TempDir()
	runner := &recordingFileRunner{output: []byte(metadata)}
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
	runner := &recordingFileRunner{output: []byte(metadata), content: []byte{}}
	u := Updater{Runner: runner}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", directory, "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "did not write") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "asset.tar.gz")); !os.IsNotExist(err) {
		t.Fatalf("empty asset was not removed: %v", err)
	}
}

func TestDownloadReleaseAssetsWithGlabAPIWritesAssetUsingLinkName(t *testing.T) {
	metadata := `{"assets":{"links":[{"name":"asset.tar.gz","url":"https://example.test/downloads/asset.tar.gz"}]}}`
	directory := t.TempDir()
	runner := &recordingFileRunner{output: []byte(metadata)}
	runner.content = []byte("downloaded")
	u := Updater{Runner: runner}
	if err := u.downloadReleaseAssetsWithGlabAPI(context.Background(), "v1.0.0", directory, "asset.tar.gz"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(directory, "asset.tar.gz"))
	if err != nil || string(got) != "downloaded" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
