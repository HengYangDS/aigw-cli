package upgrade

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecRunnerRunToFileWithEnvRejectsUnwritableDestination(t *testing.T) {
	runner := ExecRunner{}
	destination := filepath.Join(t.TempDir(), "missing", "out")
	if err := runner.RunToFileWithEnv(context.Background(), nil, destination, "echo", "hi"); err == nil || !strings.Contains(err.Error(), "open command output") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecRunnerRunToFileWithEnvPropagatesCommandFailure(t *testing.T) {
	runner := ExecRunner{}
	destination := filepath.Join(t.TempDir(), "out")
	err := runner.RunToFileWithEnv(context.Background(), nil, destination, "sh", "-c", "exit 3")
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("output file was not removed after failure: %v", statErr)
	}
}

func TestExecRunnerRunToFileWithEnvSucceeds(t *testing.T) {
	runner := ExecRunner{}
	destination := filepath.Join(t.TempDir(), "out")
	if err := runner.RunToFileWithEnv(context.Background(), nil, destination, "sh", "-c", "printf hello"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "hello" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestExecRunnerRunWithEnvPropagatesCommandFailure(t *testing.T) {
	runner := ExecRunner{}
	if _, err := runner.RunWithEnv(context.Background(), nil, "sh", "-c", "exit 4"); err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateDefaultsNilRunnerToExecRunner(t *testing.T) {
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	if _, err := u.Update(context.Background(), "0.1.0"); err == nil {
		t.Fatal("Update accepted an unconfigured release source")
	}
}

type unavailableGlabRunner struct{}

func (unavailableGlabRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, exec.ErrNotFound
}

func TestDownloadPeerAssetsRejectsUnwritableTempDirectory(t *testing.T) {
	// os.MkdirTemp("", ...) resolves the base directory through
	// os.TempDir(), which honors different environment variables per OS:
	// $TMPDIR on Unix, but %TMP%/%TEMP%/%USERPROFILE% (in that order) on
	// Windows. All of them must be pointed at a missing directory so the
	// workspace creation fails deterministically on every platform.
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("TMPDIR", missing)
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	u := Updater{Runner: &fakeCommandRunner{}}
	releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitLab}, Tag: "v1.0.0"}}
	if _, _, err := u.downloadPeerAssets(context.Background(), releases, "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "create update workspace") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadPeerAssetsPropagatesNonUnavailableFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", "token")
	u := Updater{
		HTTPClient: server.Client(),
		Runner:     &fakeCommandRunner{fail: map[string]error{"glab": errors.New("permission denied")}},
		GitLab:     ReleaseSource{Origin: server.URL, Repository: "group/project"},
	}
	releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitLab, Origin: server.URL, Repository: "group/project"}, Tag: "v1.0.0"}}
	if _, _, err := u.downloadPeerAssets(context.Background(), releases, "asset.tar.gz"); err == nil || !strings.Contains(err.Error(), "release assets failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadPeerAssetsFailsWhenAllSourcesAreUnavailable(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	u := Updater{Runner: unavailableGlabRunner{}}
	releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitLab}, Tag: "v1.0.0"}}
	_, _, err := u.downloadPeerAssets(context.Background(), releases, "asset.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "all reachable release sources failed") {
		t.Fatalf("error = %v", err)
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
	cases := []string{"/o/r", "o/r/", "o/r?x=1", "o/r#frag", "o\r\n/r"}
	for _, repository := range cases {
		source := ReleaseSource{Provider: ReleaseProviderGitLab, Origin: "https://example.test", Repository: repository}
		if err := validateReleaseSource(source); err == nil {
			t.Fatalf("repository=%q: accepted an invalid path", repository)
		}
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
	source := ReleaseSource{Provider: ReleaseProviderGitLab, Origin: "https://gitlab.example.test", Repository: "group/subgroup/project"}
	if err := validateReleaseSource(source); err != nil {
		t.Fatal(err)
	}
}
