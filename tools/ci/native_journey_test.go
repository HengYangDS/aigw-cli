package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeProductJourney(t *testing.T) {
	root := repositoryRoot(t)
	version, err := sourceVersion(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), executableName())
	build := exec.Command("go", "build", "-ldflags=-X=aigw-cli/internal/cli.Version="+version, "-o", artifact, "./cmd/aigw")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native product: %v: %s", err, output)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)

	t.Run("delayed token and client activation", func(t *testing.T) {
		journey := newNativeJourney(t, artifact, server.URL+"/v1", false)
		journey.run("setup", "--from", journey.manifest)
		journey.requireConfigContains("team-claude", "unused-claude")
		journey.requireNoClaudeProjection()

		journey.installClaudeFixture()
		journey.run("sync")
		journey.requireNoClaudeProjection()
		journey.setEnvironment("AIGW_TOKEN_TEAM", "native-journey-token")
		preview := journey.run("sync", "--dry-run", "--json")
		if !json.Valid(preview) {
			t.Fatalf("sync preview is not JSON: %s", preview)
		}
		journey.run("sync")
		journey.requireClaudeProjection()
		journey.run("check")
		journey.uninstallAndRequireOwnedFilesAbsent()
		journey.requireConfigContains("team-claude", "unused-claude")
	})

	t.Run("one selected account does not require every token", func(t *testing.T) {
		journey := newNativeJourney(t, artifact, server.URL+"/v1", true)
		journey.setEnvironment("AIGW_TOKEN_TEAM", "native-journey-token")
		journey.run("setup", "--from", journey.manifest, "--account", "team")
		journey.requireConfigContains("team-claude", "unused-claude")
		journey.requireClaudeProjection()
		journey.run("check")
		journey.uninstallAndRequireOwnedFilesAbsent()
	})
}

type journeyFixture struct {
	testing     *testing.T
	source      string
	binary      string
	root        string
	clientBin   string
	manifest    string
	config      string
	settings    string
	endpoint    string
	environment []string
}

func newNativeJourney(t *testing.T, source, endpoint string, installClient bool) *journeyFixture {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	clientBin := filepath.Join(root, "client bin")
	for _, directory := range []string{home, clientBin} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	journey := &journeyFixture{
		testing:   t,
		source:    source,
		binary:    filepath.Join(root, "installed", executableName()),
		root:      root,
		clientBin: clientBin,
		manifest:  filepath.Join(root, "team.toml"),
		settings:  filepath.Join(home, ".claude", "settings.json"),
		endpoint:  endpoint,
	}
	switch runtime.GOOS {
	case "darwin":
		journey.config = filepath.Join(home, "Library", "Application Support", "aigw", "config.toml")
	case "linux":
		journey.config = filepath.Join(root, "config", "aigw", "config.toml")
	case "windows":
		journey.config = filepath.Join(root, "appdata", "aigw", "config.toml")
	default:
		t.Fatalf("unsupported native journey platform %s", runtime.GOOS)
	}
	journey.environment = environmentWith(os.Environ(), map[string]string{
		"HOME":                home,
		"USERPROFILE":         home,
		"XDG_CONFIG_HOME":     filepath.Join(root, "config"),
		"XDG_DATA_HOME":       filepath.Join(root, "data"),
		"APPDATA":             filepath.Join(root, "appdata"),
		"LOCALAPPDATA":        filepath.Join(root, "localappdata"),
		"PATH":                clientBin,
		"AIGW_SECRET_BACKEND": "env",
		"NO_COLOR":            "1",
	})
	manifest := fmt.Sprintf("version = 3\nrecommended_default = 'team-claude'\n\n[recommended_routes]\nclaude = 'team-claude'\n\n[accounts.team]\nlabel = 'Team'\n\n[accounts.team.endpoints]\nanthropic = %q\n\n[accounts.unused]\nlabel = 'Unused'\n\n[accounts.unused.endpoints]\nanthropic = %q\n\n[profiles.team-claude]\nlabel = 'Team Claude'\naccount = 'team'\nclient = 'claude'\n\n[profiles.team-claude.models]\nclaude = 'claude-test'\n\n[profiles.unused-claude]\nlabel = 'Unused Claude'\naccount = 'unused'\nclient = 'claude'\n\n[profiles.unused-claude.models]\nclaude = 'claude-test'\n", endpoint, endpoint)
	if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if installClient {
		journey.installClaudeFixture()
	}
	journey.runWith(source, "install", "--target", journey.binary)
	journey.run("--version")
	return journey
}

func (j *journeyFixture) installClaudeFixture() {
	j.testing.Helper()
	name, content, mode := "claude", "#!/bin/sh\nprintf 'AIGW_OK\\n'\n", os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		name, content, mode = "claude.cmd", "@echo off\r\necho AIGW_OK\r\n", 0o600
	}
	if err := os.WriteFile(filepath.Join(j.clientBin, name), []byte(content), mode); err != nil {
		j.testing.Fatal(err)
	}
}

func (j *journeyFixture) setEnvironment(key, value string) {
	j.testing.Helper()
	j.environment = environmentWith(j.environment, map[string]string{key: value})
}

func (j *journeyFixture) run(args ...string) []byte {
	j.testing.Helper()
	return j.runWith(j.binary, args...)
}

func (j *journeyFixture) runWith(binary string, args ...string) []byte {
	j.testing.Helper()
	command := exec.Command(binary, args...)
	command.Env = j.environment
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		j.testing.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", binary, strings.Join(args, " "), err, stdout, stderr)
	}
	return stdout.Bytes()
}

func (j *journeyFixture) requireConfigContains(values ...string) {
	j.testing.Helper()
	requireFileContains(j.testing, j.config, values...)
}

func (j *journeyFixture) requireNoClaudeProjection() {
	j.testing.Helper()
	if _, err := os.Stat(j.settings); !os.IsNotExist(err) {
		j.testing.Fatalf("Claude settings unexpectedly exist: %v", err)
	}
}

func (j *journeyFixture) requireClaudeProjection() {
	j.testing.Helper()
	requireFileContains(j.testing, j.settings, j.endpoint, "claude-test", j.binary, "apiKeyHelper")
}

func (j *journeyFixture) uninstallAndRequireOwnedFilesAbsent() {
	j.testing.Helper()
	j.runWith(j.source, "uninstall", "--target", j.binary)
	backup := filepath.Join(filepath.Dir(j.binary), ".aigw.previous")
	if runtime.GOOS == "windows" {
		backup += ".exe"
	}
	for _, path := range []string{j.binary, backup} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			j.testing.Fatalf("uninstall retained owned file %s: %v", path, err)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "aigw.exe"
	}
	return "aigw"
}

func environmentWith(current []string, replacements map[string]string) []string {
	result := make([]string, 0, len(current)+len(replacements))
	for _, item := range current {
		key, _, found := strings.Cut(item, "=")
		if found {
			if _, replaced := replacements[key]; replaced || strings.HasPrefix(key, "AIGW_TOKEN_") {
				continue
			}
		}
		result = append(result, item)
	}
	for key, value := range replacements {
		result = append(result, key+"="+value)
	}
	return result
}

func requireFileContains(t *testing.T, path string, values ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, value := range values {
		if !bytes.Contains(data, []byte(value)) {
			t.Fatalf("%s lacks %q:\n%s", path, value, data)
		}
	}
}
