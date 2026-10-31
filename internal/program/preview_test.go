package program

import (
	"aigw-cli/internal/program/artifact"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

// previewServer serves release metadata documents by escaped path and records
// each request with any credential header that it carried.
func previewServer(t *testing.T, documents map[string]string) (*httptest.Server, func() []string) {
	t.Helper()
	var lock sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		lock.Lock()
		requests = append(requests, strings.TrimSpace(path+" "+r.Header.Get("Authorization")+r.Header.Get("Private-Token")))
		lock.Unlock()
		document, ok := documents[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, document)
	}))
	t.Cleanup(server.Close)
	return server, func() []string {
		lock.Lock()
		defer lock.Unlock()
		return slices.Clone(requests)
	}
}

// previewUpdater is an updater of a portable program in its own directory
// whose release sources are the test's, with the environment's overrides
// cleared and tokens that a preview must not send.
func previewUpdater(t *testing.T, server *httptest.Server, gitlab bool) Updater {
	t.Helper()
	for _, name := range []string{"AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY", "AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY"} {
		t.Setenv(name, "")
	}
	for _, name := range []string{"AIGW_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN"} {
		t.Setenv(name, "preview-must-not-send")
	}
	executable := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(executable, []byte("program"), 0o700); err != nil {
		t.Fatal(err)
	}
	updater := Updater{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Executable: executable, HTTPClient: server.Client(),
		GitHub: ReleaseSource{Origin: server.URL, Repository: "team/aigw"}}
	if gitlab {
		updater.GitLab = ReleaseSource{Origin: server.URL, Repository: "team/aigw"}
	}
	return updater
}

func TestPreviewReadsPublicMetadataAlone(t *testing.T) {
	server, requests := previewServer(t, map[string]string{
		"/repos/team/aigw/releases/latest":                       `{"tag_name":"v0.3.0"}`,
		"/api/v4/projects/team%2Faigw/releases/permalink/latest": `{"tag_name":"v0.3.0"}`,
	})
	updater := previewUpdater(t, server, true)
	result, err := updater.Preview(context.Background(), "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.State != TransitionAvailable || result.Version != "0.3.0" {
		t.Fatalf("preview = %+v", result)
	}
	want := []string{"/api/v4/projects/team%2Faigw/releases/permalink/latest", "/repos/team/aigw/releases/latest"}
	if got := requests(); !slices.Equal(got, want) {
		t.Fatalf("requests = %q, want only the two metadata documents without a credential", got)
	}
	entries, err := os.ReadDir(filepath.Dir(updater.Executable))
	if err != nil || len(entries) != 1 {
		t.Fatalf("preview wrote beside the program: %v %v", entries, err)
	}
}

func TestPreviewFollowsTheUpdateRules(t *testing.T) {
	t.Run("prerelease only", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{
			"/repos/team/aigw/releases?per_page=100": `[{"tag_name":"v0.3.0-rc.1","prerelease":true,"published_at":"2026-10-31T00:00:00Z"}]`,
		})
		result, err := previewUpdater(t, server, false).Preview(context.Background(), "0.2.0")
		if err != nil || result.State != TransitionAvailable || result.Version != "0.3.0-rc.1" {
			t.Fatalf("preview = %+v, %v", result, err)
		}
	})
	t.Run("current", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{"/repos/team/aigw/releases/latest": `{"tag_name":"v0.2.0"}`})
		result, err := previewUpdater(t, server, false).Preview(context.Background(), "0.2.0")
		if err != nil || result.State != TransitionCurrent {
			t.Fatalf("preview = %+v, %v", result, err)
		}
	})
	t.Run("older", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{"/repos/team/aigw/releases/latest": `{"tag_name":"v0.1.0"}`})
		if _, err := previewUpdater(t, server, false).Preview(context.Background(), "0.2.0"); err == nil || !strings.Contains(err.Error(), "older release v0.1.0") {
			t.Fatalf("preview error = %v", err)
		}
	})
	t.Run("private sources", func(t *testing.T) {
		server, _ := previewServer(t, nil)
		if _, err := previewUpdater(t, server, true).Preview(context.Background(), "0.2.0"); err == nil || !strings.Contains(err.Error(), "without authentication") {
			t.Fatalf("preview error = %v", err)
		}
	})
	t.Run("rollback directory", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{"/repos/team/aigw/releases/latest": `{"tag_name":"v0.3.0"}`})
		updater := previewUpdater(t, server, false)
		if err := os.Mkdir(RollbackPath(updater.Executable), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := updater.Preview(context.Background(), "0.2.0"); err == nil || !strings.Contains(err.Error(), "rollback copy path") {
			t.Fatalf("preview error = %v", err)
		}
		if result, err := updater.Preview(context.Background(), "0.3.0"); err != nil || result.State != TransitionCurrent {
			t.Fatalf("current preview = %+v, %v", result, err)
		}
	})
	t.Run("sources disagree", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{
			"/repos/team/aigw/releases/latest":                       `{"tag_name":"v0.3.0"}`,
			"/api/v4/projects/team%2Faigw/releases/permalink/latest": `{"tag_name":"v0.4.0"}`,
		})
		if _, err := previewUpdater(t, server, true).Preview(context.Background(), "0.2.0"); err == nil || !strings.Contains(err.Error(), "disagree") {
			t.Fatalf("preview error = %v", err)
		}
	})
	t.Run("empty tags", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{
			"/repos/team/aigw/releases/latest":                       `{"tag_name":""}`,
			"/api/v4/projects/team%2Faigw/releases/permalink/latest": `{"tag_name":""}`,
		})
		for _, gitlab := range []bool{false, true} {
			updater := previewUpdater(t, server, gitlab)
			if !gitlab {
				updater.GitHub, updater.GitLab = ReleaseSource{Origin: server.URL, Repository: "team/aigw"}, ReleaseSource{}
			} else {
				updater.GitHub = ReleaseSource{}
			}
			if _, err := updater.Preview(context.Background(), "0.2.0"); err == nil || !strings.Contains(err.Error(), "no AIGW release is available") {
				t.Fatalf("gitlab %v preview error = %v", gitlab, err)
			}
		}
	})
	t.Run("unreachable source", func(t *testing.T) {
		server, _ := previewServer(t, nil)
		updater := previewUpdater(t, server, false)
		server.Close()
		if _, err := updater.Preview(context.Background(), "0.2.0"); err == nil || !strings.Contains(err.Error(), "without authentication") {
			t.Fatalf("preview error = %v", err)
		}
	})
	t.Run("invalid current version", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{"/repos/team/aigw/releases/latest": `{"tag_name":"v0.3.0"}`})
		if _, err := previewUpdater(t, server, false).Preview(context.Background(), "not-a-version"); err == nil {
			t.Fatal("an invalid current version was compared")
		}
	})
	t.Run("malformed metadata", func(t *testing.T) {
		server, _ := previewServer(t, map[string]string{"/repos/team/aigw/releases/latest": `{`})
		if _, err := previewUpdater(t, server, false).Preview(context.Background(), "0.2.0"); err == nil || !strings.Contains(err.Error(), "public release metadata failed") {
			t.Fatalf("preview error = %v", err)
		}
	})
}

func TestPreviewCandidateReadsTheArchiveNameAlone(t *testing.T) {
	server, requests := previewServer(t, nil)
	updater := previewUpdater(t, server, false)
	directory := t.TempDir()
	archive := filepath.Join(directory, (artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}).ArchiveName("0.3.0"))
	checksums := filepath.Join(directory, "checksums.txt")
	for _, path := range []string{archive, checksums} {
		if err := os.WriteFile(path, []byte("not an archive"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := updater.PreviewCandidate("0.2.0", CandidateArchive{ArchivePath: archive, ChecksumsPath: checksums})
	if err != nil || result.State != TransitionAvailable || result.Version != "0.3.0" {
		t.Fatalf("candidate preview = %+v, %v", result, err)
	}
	if got := requests(); len(got) != 0 {
		t.Fatalf("candidate preview sent requests: %q", got)
	}
	if _, err := updater.PreviewCandidate("0.2.0", CandidateArchive{ArchivePath: archive + ".missing", ChecksumsPath: checksums}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing candidate error = %v", err)
	}
	if _, err := updater.PreviewCandidate("0.2.0", CandidateArchive{ArchivePath: archive, ChecksumsPath: directory}); err == nil || !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("directory checksums error = %v", err)
	}
	if _, err := updater.PreviewCandidate("0.2.0", CandidateArchive{ArchivePath: archive}); err == nil || !strings.Contains(err.Error(), "requires both") {
		t.Fatalf("missing checksums path error = %v", err)
	}
	if _, err := updater.PreviewCandidate("0.2.0", CandidateArchive{ArchivePath: checksums, ChecksumsPath: checksums}); err == nil {
		t.Fatal("an archive without a release name was previewed")
	}
}
