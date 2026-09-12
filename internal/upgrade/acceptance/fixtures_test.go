package acceptance_test

import (
	"aigw-cli/internal/process"
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
		"GITLAB_HOST":                    "",
		"GLAB_CONFIG_DIR":                filepath.Join(isolated, "glab"),
		"XDG_CONFIG_HOME":                filepath.Join(isolated, "config"),
		"HOME":                           filepath.Join(isolated, "home"),
	}
	for _, directory := range []string{values["GLAB_CONFIG_DIR"], values["XDG_CONFIG_HOME"], values["HOME"]} {
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
	if err := os.RemoveAll(isolated); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "remove isolated release-test home: %v\n", err)
		code = 1
	}
	os.Exit(code)
}

type releaseRunner struct {
	download func(directory, asset string) error
	archive  []byte
	checksum string
	tag      string
	version  string
	calls    [][]string
}

type unavailableRunner struct {
	calls   [][]string
	version string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func (r *unavailableRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	name, args := plan.Executable, plan.Args
	r.calls = append(r.calls, append([]string{name}, args...))
	if filepath.IsAbs(name) && slices.Equal(args, []string{"--version"}) {
		version := r.version
		if version == "" {
			version = "0.2.0"
		}
		return []byte("aigw version " + version + "\n"), nil
	}
	return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func (r *releaseRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	name, args := plan.Executable, plan.Args
	r.calls = append(r.calls, append([]string{name}, args...))
	if filepath.IsAbs(name) && slices.Equal(args, []string{"--version"}) {
		version := r.version
		if version == "" {
			version = "0.2.0"
		}
		return []byte("aigw version " + version + "\n"), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		tag := r.tag
		if tag == "" {
			tag = "v0.2.0"
		}
		if slices.Contains(args, "--jq") {
			return []byte(tag + "\n"), nil
		}
		return []byte(`[{"tag_name":"` + tag + `"}]`), nil
	}
	if len(args) < 2 || args[0] != "release" || args[1] != "download" {
		switch name {
		case "open", "sudo", "msiexec":
			return []byte("ok"), nil
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	dir, asset := "", ""
	for i, arg := range args {
		if arg == "--dir" {
			dir = args[i+1]
		}
		if arg == "--asset-name" {
			asset = args[i+1]
		}
	}
	if r.download != nil {
		return nil, r.download(dir, asset)
	}
	if asset == "checksums.txt" {
		return nil, os.WriteFile(filepath.Join(dir, asset), []byte(r.checksum), 0o600)
	}
	return nil, os.WriteFile(filepath.Join(dir, asset), r.archive, 0o600)
}

func (r *releaseRunner) downloaded(asset string) bool {
	for _, call := range r.calls {
		for i, part := range call {
			if part == "--asset-name" && i+1 < len(call) && call[i+1] == asset {
				return true
			}
		}
	}
	return false
}

func calledCommand(calls [][]string, prefix ...string) bool {
	return slices.ContainsFunc(calls, func(call []string) bool {
		return len(call) >= len(prefix) && slices.Equal(call[:len(prefix)], prefix)
	})
}

func containsSequence(values []string, want ...string) bool {
	for i := 0; i+len(want) <= len(values); i++ {
		if slices.Equal(values[i:i+len(want)], want) {
			return true
		}
	}
	return false
}

func tarGz(t *testing.T, name string, payloads ...[]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for _, data := range payloads {
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
