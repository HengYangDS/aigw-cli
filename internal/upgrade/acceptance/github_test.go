package acceptance_test

import (
	"aigw-cli/internal/process"
	"aigw-cli/internal/upgrade"
	"context"
	"crypto/sha256"
	"fmt"
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

type githubReleaseRunner struct {
	archive  []byte
	checksum string
	calls    [][]string
}

func (r *githubReleaseRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	name, args := plan.Executable, plan.Args
	r.calls = append(r.calls, append([]string{name}, args...))
	if filepath.IsAbs(name) && slices.Equal(args, []string{"--version"}) {
		return []byte("aigw version 0.2.0-rc.1\n"), nil
	}
	if name == "glab" {
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	if name != "gh" || len(args) < 2 {
		return nil, fmt.Errorf("unexpected command: %s %v", name, args)
	}
	if args[0] == "api" {
		switch args[1] {
		case "repos/example-owner/aigw-cli/releases/latest":
			return nil, fmt.Errorf("gh api failed: HTTP 404")
		case "repos/example-owner/aigw-cli/releases?per_page=100":
			return []byte(`[{"tag_name":"v0.2.0-rc.1","prerelease":true,"published_at":"2026-07-15T00:00:00Z"}]`), nil
		}
	}
	if args[0] != "release" || args[1] != "download" {
		return nil, fmt.Errorf("unexpected GH CLI command: %v", args)
	}
	directory, pattern := "", ""
	for index, arg := range args {
		if arg == "--dir" && index+1 < len(args) {
			directory = args[index+1]
		}
		if arg == "--pattern" && index+1 < len(args) {
			pattern = args[index+1]
		}
	}
	if directory == "" || pattern == "" {
		return nil, fmt.Errorf("GH release download lacks directory or pattern: %v", args)
	}
	data := r.archive
	if pattern == "checksums.txt" {
		data = []byte(r.checksum)
	}
	return nil, os.WriteFile(filepath.Join(directory, pattern), data, 0o600)
}

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
			_, _ = fmt.Fprintf(w, "%x  %s\n", sum, archiveName)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", server.URL)
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "example-owner/aigw-cli")
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &unavailableRunner{version: "0.2.0-rc.1"}, HTTPClient: server.Client()}
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

func TestUpdateUsesAuthenticatedGHCLIForPrivatePublishedPrerelease(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0-rc.1_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0-rc.1_darwin_arm64.tar.gz"
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &githubReleaseRunner{archive: archive, checksum: checksum}
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
	if !calledCommand(runner.calls, "gh", "api", "repos/example-owner/aigw-cli/releases/latest") ||
		!calledCommand(runner.calls, "gh", "api", "repos/example-owner/aigw-cli/releases?per_page=100") ||
		!calledCommand(runner.calls, "gh", "release", "download") {
		t.Fatalf("GH CLI fallback calls = %v", runner.calls)
	}
}

func TestUpdateDoesNotUseGHCLIOutsideTheSupportedFallback(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		status int
	}{
		{name: "custom origin", origin: "https://github.example.test", status: http.StatusNotFound},
		{name: "non-not-found failure", origin: "https://github.com", status: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &githubReleaseRunner{}
			statusText := http.StatusText(test.status)
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: test.status,
					Status:     fmt.Sprintf("%d %s", test.status, statusText),
					Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"message":%q}`, statusText))),
					Header:     make(http.Header),
					Request:    request,
				}, nil
			})}
			u := upgrade.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     runner,
				HTTPClient: client,
				GitHub:     upgrade.ReleaseSource{Provider: upgrade.ReleaseProviderGitHub, Origin: test.origin, Repository: "example-owner/aigw-cli"},
			}
			_, err := u.Update(context.Background(), "0.1.0")
			if err == nil || !strings.Contains(err.Error(), fmt.Sprint(test.status)) {
				t.Fatalf("error = %v", err)
			}
			for _, call := range runner.calls {
				if call[0] == "gh" {
					t.Fatalf("unsupported fallback unexpectedly invoked gh: %v", runner.calls)
				}
			}
		})
	}
}
