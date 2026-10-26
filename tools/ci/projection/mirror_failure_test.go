package projection

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGitLabUnixMiseMirrorFailureStaysOnSelectedPeer(t *testing.T) {
	for _, test := range []struct {
		name     string
		metadata bool
	}{
		{"whole mirror absent", false}, {"asset absent with metadata", true},
	} {
		t.Run(test.name, func(t *testing.T) { testGitLabUnixMiseMirrorFailure(t, test.metadata) })
	}
}

func testGitLabUnixMiseMirrorFailure(t *testing.T, metadata bool) {
	if runtime.GOOS == "windows" {
		return // The Unix projection requires a POSIX shell.
	}
	repository := filepath.Clean(filepath.Join("..", "..", ".."))
	prepare, cleanup := unixMiseMirrorCommands(t)
	project, selected := nativeMiseMirrorInputs(t, repository)
	mirrorPath := "/api/v4/projects/456/" + lockedMiseMirrorPath(t, project)
	t.Setenv("AIGW_MISE_DATA_ROOT", t.TempDir())
	assetID, err := strconv.ParseUint(path.Base(selected.URLAPI), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	var requests []string
	var mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requests = append(requests, r.Method+" "+r.Host+r.URL.Path)
		mutex.Unlock()
		if metadata && r.Header.Get("Accept") == "application/vnd.github+json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": assetID, "name": path.Base(selected.URL), "url": selected.URLAPI, "browser_download_url": selected.URL})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	environment := slices.DeleteFunc(os.Environ(), func(value string) bool {
		name, _, _ := strings.Cut(value, "=")
		return name == "AIGW_MISE_DATA_ROOT" || strings.HasPrefix(name, "MISE_") || strings.HasPrefix(name, "__MISE_") ||
			strings.HasPrefix(name, "GH_") || strings.HasPrefix(name, "GITHUB_") || strings.HasPrefix(name, "GITLAB_")
	})
	environment = append(environment,
		"AIGW_MISE_DATA_ROOT="+t.TempDir(),
		"HOME="+project, "XDG_CONFIG_HOME="+filepath.Join(project, ".config"),
		"MISE_TRUSTED_CONFIG_PATHS="+project, "MISE_CONFIG_DIR="+filepath.Join(project, ".config", "mise"),
		"MISE_CEILING_PATHS="+filepath.Dir(project),
		"MISE_GLOBAL_CONFIG_FILE="+filepath.Join(project, ".config", "mise", "config.toml"), "MISE_SYSTEM_CONFIG_DIR="+filepath.Join(project, "system"),
		"MISE_GITHUB_CREDENTIAL_COMMAND=", "MISE_GITLAB_CREDENTIAL_COMMAND=",
		"MISE_GITHUB_GH_CLI_TOKENS=false", "MISE_GITLAB_GLAB_CLI_TOKENS=false",
		"MISE_HTTP_RETRIES=0", "MISE_HTTP_TIMEOUT=2s", "MISE_USE_VERSIONS_HOST=false",
		"AIGW_TOOL_SOURCE=peer", "CI_API_V4_URL="+server.URL+"/api/v4", "CI_PROJECT_ID=456",
		"CI_SERVER_HOST="+strings.TrimPrefix(server.URL, "http://"), "CI_JOB_ID=123", "CI_JOB_TOKEN=fixture-only",
		"HTTP_PROXY="+server.URL, "HTTPS_PROXY="+server.URL, "ALL_PROXY="+server.URL,
		"NO_PROXY=127.0.0.1", "http_proxy="+server.URL, "https_proxy="+server.URL, "all_proxy="+server.URL, "no_proxy=127.0.0.1",
	)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "sh", "-eu", "-c", cleanup)
		command.Dir, command.Env = project, environment
		if output, err := command.CombinedOutput(); err != nil {
			t.Errorf("native mirror cleanup: %v\n%s", err, output)
		}
		if _, err := os.Stat(filepath.Join(project, "build", "tmp", ".aigw-mise-mirror-123")); !os.IsNotExist(err) {
			t.Errorf("owned mirror remains: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "sh", "-eu", "-c", prepare+"\ntest \"$MISE_DATA_DIR\" = \"$mirror_dir/mise-data\"\nexec mise install --locked github:anchore/syft --jobs=1")
	command.Dir, command.Env, command.WaitDelay = project, environment, time.Second
	output, err := command.CombinedOutput()
	if err == nil || ctx.Err() != nil {
		t.Errorf("missing mirror must fail without expiry: %v\n%s", err, output)
	}
	assertMirrorFailureDiagnostics(t, string(output), metadata, selected.URLAPI, selected.URL)
	mutex.Lock()
	defer mutex.Unlock()
	assertMirrorFailureRequests(t, requests, server.URL, mirrorPath, selected.URL, selected.URLAPI)
}

func assertMirrorFailureDiagnostics(t *testing.T, output string, metadata bool, assetAPI, assetURL string) {
	t.Helper()
	if !strings.Contains(output, "404 Not Found") || !strings.Contains(output, "installed 0 tools") {
		t.Errorf("missing mirror must report bounded 404 with zero tools installed: %s", output)
	}
	warnings := 0
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, "mise WARN") {
			continue
		}
		warnings++
		want := "mise WARN  could not confirm GitHub release asset " + assetAPI + " is " + assetURL + ": HTTP status client error (404"
		if metadata || !strings.HasPrefix(strings.TrimSpace(line), want) {
			t.Errorf("unexpected native mirror diagnostic: %s", line)
		}
	}
	if !metadata && warnings != 1 {
		t.Errorf("absent asset metadata must retain its one native warning: %s", output)
	}
}

func assertMirrorFailureRequests(t *testing.T, requests []string, serverURL, mirrorPath string, upstreams ...string) {
	t.Helper()
	for _, request := range requests {
		method, target, _ := strings.Cut(request, " ")
		if (method != http.MethodGet && method != http.MethodHead) || !strings.HasPrefix(target, strings.TrimPrefix(serverURL, "http://")+mirrorPath) {
			t.Errorf("native acquisition escaped selected peer: %s", request)
		}
	}
	for _, upstream := range upstreams {
		parsed, err := url.Parse(upstream)
		if err != nil {
			t.Fatalf("unexpected native locked URL: %q, %v", upstream, err)
		}
		want := strings.TrimPrefix(serverURL, "http://") + strings.TrimSuffix(mirrorPath, "/") + parsed.Path
		if !slices.ContainsFunc(requests, func(request string) bool { _, target, _ := strings.Cut(request, " "); return target == want }) {
			t.Errorf("native locked request was not observed: %s in %v", want, requests)
		}
	}
}
