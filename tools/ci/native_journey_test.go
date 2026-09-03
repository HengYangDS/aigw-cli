package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/secrets"
)

func TestNativeProductJourney(t *testing.T) {
	root := repositoryRoot(t)
	newVersion, err := sourceVersion(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	const oldVersion = "0.0.0"
	artifact := buildNativeProgram(t, root, oldVersion)

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)

	t.Run("delayed token and client activation", func(t *testing.T) {
		journey := newNativeJourney(t, artifact, server.URL+"/v1", false)
		journey.run("setup", "--from", journey.manifest)
		journey.requireConfigContains("native-system-keyring-probe-claude", "unused-claude")
		journey.requireNoClaudeProjection()

		journey.installClaudeFixture()
		journey.run("sync")
		journey.requireNoClaudeProjection()
		journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
		preview := journey.run("sync", "--dry-run", "--json")
		if !json.Valid(preview) {
			t.Fatalf("sync preview is not JSON: %s", preview)
		}
		journey.run("sync")
		journey.requireClaudeProjection()
		journey.run("check")
		journey.uninstallAndRequireOwnedFilesAbsent()
		journey.requireConfigContains("native-system-keyring-probe-claude", "unused-claude")
	})

	t.Run("one selected account does not require every token", func(t *testing.T) {
		journey := newNativeJourney(t, artifact, server.URL+"/v1", true)
		journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
		journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
		journey.requireConfigContains("native-system-keyring-probe-claude", "unused-claude")
		journey.requireClaudeProjection()
		journey.run("check")
		var diagnosis struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(journey.run("doctor", "--json"), &diagnosis); err != nil {
			t.Fatal(err)
		}
		if !diagnosis.OK {
			t.Fatal("doctor rejected a healthy partially connected catalogue")
		}
		journey.uninstallAndRequireOwnedFilesAbsent()
	})

	t.Run("released artifact lifecycle", func(t *testing.T) {
		runNativeReleaseLifecycle(t, root, artifact, newVersion, server.URL+"/v1")
	})

	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") == "1" {
		t.Run("system credential store", func(t *testing.T) {
			journey := newNativeJourney(t, artifact, server.URL+"/v1", true)
			journey.enableSystemCredentialStore()
			store := secrets.NewKeyringStore()
			const (
				token       = "native-system-keyring-token"
				replacement = "native-system-keyring-replacement"
			)
			backend := secrets.BackendSelection{
				Kind:         "keyring",
				Availability: "available",
				Mutability:   "read_write",
				Persistence:  "persisted",
			}
			t.Cleanup(func() {
				if err := store.Delete("native-system-keyring-probe"); err != nil {
					t.Errorf("clean system credential store: %v", err)
				}
			})
			journey.runInput(token+"\n", "setup", "--from", journey.manifest, "--account", "native-system-keyring-probe", "--token-stdin")
			journey.requireCredentialBackend(token, backend)
			if got := strings.TrimSpace(string(journey.run("credential", "claude"))); got != token {
				t.Fatalf("credential = %q", got)
			}
			journey.runInput(replacement+"\n", "rotate", "native-system-keyring-probe", "--token-stdin")
			journey.requireCredentialBackend(replacement, backend)
			if got, err := store.Get("native-system-keyring-probe"); err != nil || got != replacement {
				t.Fatalf("replaced credential = %q, %v", got, err)
			}
			if err := store.Delete("native-system-keyring-probe"); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get("native-system-keyring-probe"); !errors.Is(err, secrets.ErrNotFound) {
				t.Fatalf("deleted credential remains: %v", err)
			}
			journey.uninstallAndRequireOwnedFilesAbsent()
		})
	}

	if runtime.GOOS == "linux" && os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") == "1" {
		t.Run("secure file fallback without session bus", func(t *testing.T) {
			journey := newNativeJourney(t, artifact, server.URL+"/v1", true)
			journey.enableSystemCredentialStore()
			journey.setEnvironment(
				"DBUS_SESSION_BUS_ADDRESS",
				"unix:path="+filepath.Join(journey.root, "missing-session-bus.sock"),
			)
			const token = "native-secure-file-token"
			journey.runInput(token+"\n", "setup", "--from", journey.manifest, "--account", "native-system-keyring-probe", "--token-stdin")
			journey.requireCredentialBackend(token, secrets.BackendSelection{
				Kind:         "file",
				Availability: "available",
				Mutability:   "read_write",
				Persistence:  "persisted",
			})
			if got := strings.TrimSpace(string(journey.run("credential", "claude"))); got != token {
				t.Fatalf("credential = %q", got)
			}
			backend := filepath.Join(journey.root, "data", "aigw", "secrets", "backend")
			if got := strings.TrimSpace(string(readFile(t, backend))); got != "file" {
				t.Fatalf("persisted backend = %q, want file", got)
			}
			journey.uninstallAndRequireOwnedFilesAbsent()
		})
	}
}

func buildNativeProgram(t *testing.T, root, version string) string {
	t.Helper()
	artifact := filepath.Join(t.TempDir(), executableName())
	build := exec.Command("go", "build", "-ldflags=-X=aigw-cli/internal/cli.Version="+version, "-o", artifact, "./cmd/aigw")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native product %s: %v: %s", version, err, output)
	}
	return artifact
}

func TestSystemCredentialEnvironmentPreservesDarwinLoginHome(t *testing.T) {
	temporaryHome := filepath.Join(t.TempDir(), "isolated-home")
	hostHome := "/Users/runner"
	environment, err := systemCredentialEnvironment(
		"darwin",
		[]string{"HOME=" + temporaryHome, "USERPROFILE=" + temporaryHome, "AIGW_SECRET_BACKEND=env"},
		hostHome,
		"ephemeral-host",
	)
	if err != nil {
		t.Fatal(err)
	}
	values := environmentValues(environment)
	if values["HOME"] != hostHome || values["USERPROFILE"] != hostHome {
		t.Fatalf("Darwin system credential environment = %#v, want login home %q", values, hostHome)
	}
	if _, present := values["AIGW_SECRET_BACKEND"]; present {
		t.Fatal("system credential environment retained the environment backend")
	}
}

func TestSystemCredentialEnvironmentRequiresEphemeralDarwinHost(t *testing.T) {
	for _, scope := range []string{"", "persistent-host"} {
		if _, err := systemCredentialEnvironment("darwin", []string{"HOME=/tmp/isolated"}, "/Users/runner", scope); err == nil {
			t.Fatalf("Darwin system credential environment admitted scope %q", scope)
		}
	}
	if _, err := systemCredentialEnvironment("darwin", nil, "", "ephemeral-host"); err == nil {
		t.Fatal("Darwin system credential environment admitted an empty login home")
	}
}

func (j *journeyFixture) enableSystemCredentialStore() {
	j.testing.Helper()
	environment, err := systemCredentialEnvironment(runtime.GOOS, j.environment, os.Getenv("HOME"), os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"))
	if err != nil {
		j.testing.Fatal(err)
	}
	j.environment = environment
}

func systemCredentialEnvironment(goos string, current []string, hostHome, scope string) ([]string, error) {
	environment := environmentWithout(current, "AIGW_SECRET_BACKEND")
	if goos != "darwin" {
		return environment, nil
	}
	if scope != "ephemeral-host" {
		return nil, fmt.Errorf("Darwin system credential verification requires an explicitly ephemeral host")
	}
	if hostHome == "" {
		return nil, fmt.Errorf("Darwin system credential verification requires the login HOME")
	}
	return environmentWith(environment, map[string]string{
		"HOME":        hostHome,
		"USERPROFILE": hostHome,
	}), nil
}

func environmentValues(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, present := strings.Cut(entry, "=")
		if present {
			values[key] = value
		}
	}
	return values
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
	manifest := fmt.Sprintf("version = 4\n\n[recommended_routes]\nclaude = 'native-system-keyring-probe-claude'\n\n[accounts.native-system-keyring-probe]\nlabel = 'Native System Keyring Probe'\n\n[accounts.native-system-keyring-probe.endpoints]\nanthropic = %q\n\n[accounts.unused]\nlabel = 'Unused'\n\n[accounts.unused.endpoints]\nanthropic = %q\n\n[profiles.native-system-keyring-probe-claude]\nlabel = 'Native System Keyring Probe Claude'\naccount = 'native-system-keyring-probe'\nclient = 'claude'\nmodel = 'claude-test'\n\n[profiles.unused-claude]\nlabel = 'Unused Claude'\naccount = 'unused'\nclient = 'claude'\nmodel = 'claude-test'\n", endpoint, endpoint)
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
	return j.runWithInput(j.binary, "", args...)
}

func (j *journeyFixture) runWith(binary string, args ...string) []byte {
	j.testing.Helper()
	return j.runWithInput(binary, "", args...)
}

func (j *journeyFixture) runInput(input string, args ...string) []byte {
	j.testing.Helper()
	return j.runWithInput(j.binary, input, args...)
}

func (j *journeyFixture) runWithInput(binary, input string, args ...string) []byte {
	j.testing.Helper()
	command := exec.Command(binary, args...)
	command.Env = j.environment
	command.Stdin = strings.NewReader(input)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		j.testing.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", binary, strings.Join(args, " "), err, stdout, stderr)
	}
	return stdout.Bytes()
}

func (j *journeyFixture) requireCredentialBackend(token string, want secrets.BackendSelection) {
	j.testing.Helper()
	for _, command := range [][]string{{"status", "--json"}, {"doctor", "--json"}} {
		output := j.run(command...)
		if bytes.Contains(output, []byte(token)) {
			j.testing.Fatalf("%s disclosed the credential", strings.Join(command, " "))
		}
		var result struct {
			CredentialBackend secrets.BackendSelection `json:"credential_backend"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			j.testing.Fatalf("decode %s: %v", strings.Join(command, " "), err)
		}
		if result.CredentialBackend != want {
			j.testing.Fatalf("%s credential backend = %#v, want %#v", strings.Join(command, " "), result.CredentialBackend, want)
		}
	}
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
	requireFileContains(j.testing, j.settings, j.endpoint, "claude-test", "apiKeyHelper")
	data, err := os.ReadFile(j.settings)
	if err != nil {
		j.testing.Fatal(err)
	}
	var settings struct {
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		j.testing.Fatalf("decode Claude settings: %v", err)
	}
	if !strings.Contains(settings.APIKeyHelper, j.binary) {
		j.testing.Fatalf("Claude apiKeyHelper %q lacks %q", settings.APIKeyHelper, j.binary)
	}
}

func (j *journeyFixture) uninstallAndRequireOwnedFilesAbsent() {
	j.testing.Helper()
	j.runWith(j.source, "uninstall", "--target", j.binary)
	j.requireOwnedFilesAbsent()
}

func (j *journeyFixture) requireOwnedFilesAbsent() {
	j.testing.Helper()
	backup := filepath.Join(filepath.Dir(j.binary), ".aigw.previous")
	if runtime.GOOS == "windows" {
		backup += ".exe"
	}
	for _, path := range []string{j.binary, backup} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			j.testing.Fatalf("uninstall retained owned file %s: %v", path, err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(j.binary))
	if err != nil {
		j.testing.Fatal(err)
	}
	if len(entries) != 0 {
		j.testing.Fatalf("uninstall retained lifecycle residue: %#v", entryNames(entries))
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

func environmentWithout(current []string, removed ...string) []string {
	keys := make(map[string]struct{}, len(removed))
	for _, key := range removed {
		keys[key] = struct{}{}
	}
	result := make([]string, 0, len(current))
	for _, item := range current {
		key, _, found := strings.Cut(item, "=")
		if _, remove := keys[key]; found && remove {
			continue
		}
		result = append(result, item)
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

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
