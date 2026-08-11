package upgrade_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const testReleaseProject = "example-group/example-project"

func TestMain(m *testing.M) {
	isolated, err := os.MkdirTemp("", "aigw-upgrade-test-")
	if err != nil {
		panic(err)
	}

	values := map[string]string{
		"AIGW_GITLAB_RELEASE_ORIGIN":     "https://gitlab.example.test",
		"AIGW_GITLAB_RELEASE_REPOSITORY": testReleaseProject,
		"AIGW_GITHUB_RELEASE_ORIGIN":     "",
		"AIGW_GITHUB_RELEASE_REPOSITORY": "",
		"AIGW_GITHUB_TOKEN":              "",
		"GITHUB_TOKEN":                   "",
		"GH_TOKEN":                       "",
		"GITLAB_TOKEN":                   "",
		"GL_HOST":                        "",
		"GL_CONFIG_DIR":                  filepath.Join(isolated, "glab"),
		"XDG_CONFIG_HOME":                filepath.Join(isolated, "config"),
		"HOME":                           filepath.Join(isolated, "home"),
	}
	for _, directory := range []string{values["GL_CONFIG_DIR"], values["XDG_CONFIG_HOME"], values["HOME"]} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			panic(err)
		}
	}
	for name, value := range values {
		if err := os.Setenv(name, value); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(isolated)
	os.Exit(code)
}

type fakeRunner struct {
	archive  []byte
	checksum string
	tag      string
	calls    [][]string
}

type missingGlabRunner struct {
	calls [][]string
}

type emptyDownloadRunner struct {
	calls [][]string
}

type missingDownloadRunner struct {
	calls [][]string
}

type glabAPIAssetRunner struct {
	archive       []byte
	checksum      string
	archiveURL    string
	checksumURL   string
	archiveLabel  string
	checksumLabel string
	calls         [][]string
	fileCalls     [][]string
}

type githubKeyringRunner struct {
	archive  []byte
	checksum string
	calls    [][]string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func (r *missingGlabRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func (r *githubKeyringRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
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
	if args[0] == "release" && args[1] == "download" {
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
	return nil, fmt.Errorf("unexpected GH CLI command: %v", args)
}

func (r *githubKeyringRunner) called(prefix ...string) bool {
	for _, call := range r.calls {
		if len(call) < len(prefix) {
			continue
		}
		match := true
		for index, want := range prefix {
			if call[index] != want {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (r *emptyDownloadRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		return []byte("v0.2.0\n"), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func (r *missingDownloadRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		return []byte("v0.2.0\n"), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		return nil, os.ErrNotExist
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func (r *glabAPIAssetRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		return []byte("v0.2.0\n"), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		return nil, nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "projects/example-group%2Fexample-project/releases/v0.2.0" {
		archiveLabel := r.archiveLabel
		if archiveLabel == "" {
			archiveLabel = "aigw_0.2.0_darwin_arm64.tar.gz"
		}
		checksumLabel := r.checksumLabel
		if checksumLabel == "" {
			checksumLabel = "checksums.txt"
		}
		return []byte(fmt.Sprintf(`{"assets":{"links":[{"name":%q,"url":%q},{"name":%q,"url":%q}]}}`, archiveLabel, r.archiveURL, checksumLabel, r.checksumURL)), nil
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func (r *glabAPIAssetRunner) RunToFile(_ context.Context, destination, name string, args ...string) error {
	r.fileCalls = append(r.fileCalls, append([]string{name}, args...))
	if name != "glab" || len(args) != 2 || args[0] != "api" {
		return fmt.Errorf("unexpected file command: %s %v", name, args)
	}
	switch args[1] {
	case r.archiveURL:
		return os.WriteFile(destination, r.archive, 0o600)
	case r.checksumURL:
		return os.WriteFile(destination, []byte(r.checksum), 0o600)
	default:
		return fmt.Errorf("unexpected API asset URL: %s", args[1])
	}
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		tag := r.tag
		if tag == "" {
			tag = "v0.2.0"
		}
		if contains(args, "--jq") {
			return []byte(tag + "\n"), nil
		}
		return []byte(`[{"tag_name":"` + tag + `"}]`), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		dir, asset := "", ""
		for i, arg := range args {
			if arg == "--dir" {
				dir = args[i+1]
			}
			if arg == "--asset-name" {
				asset = args[i+1]
			}
		}
		if asset == "checksums.txt" {
			return nil, os.WriteFile(filepath.Join(dir, asset), []byte(r.checksum), 0o600)
		}
		return nil, os.WriteFile(filepath.Join(dir, asset), r.archive, 0o600)
	}
	switch name {
	case "open", "sudo", "msiexec":
		return []byte("ok"), nil
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func (r *fakeRunner) downloaded(asset string) bool {
	for _, call := range r.calls {
		for i, part := range call {
			if part == "--asset-name" && i+1 < len(call) && call[i+1] == asset {
				return true
			}
		}
	}
	return false
}

func (r *fakeRunner) called(prefix ...string) bool {
	for _, call := range r.calls {
		if len(call) < len(prefix) {
			continue
		}
		ok := true
		for i := range prefix {
			if call[i] != prefix[i] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsSequence(values []string, want ...string) bool {
	for i := 0; i+len(want) <= len(values); i++ {
		match := true
		for j := range want {
			if values[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func copyFixtureFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o755)
}

func fixtureFilesEqual(left, right string) error {
	leftData, err := os.ReadFile(left)
	if err != nil {
		return err
	}
	rightData, err := os.ReadFile(right)
	if err != nil {
		return err
	}
	if !bytes.Equal(leftData, rightData) {
		return fmt.Errorf("contents differ")
	}
	return nil
}

func tarGzEntries(t *testing.T, entries ...[]byte) []byte {
	t.Helper()
	if len(entries)%2 != 0 {
		t.Fatal("tar fixture needs name/data pairs")
	}
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for index := 0; index < len(entries); index += 2 {
		name, data := string(entries[index]), entries[index+1]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func tarGz(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
