package main

import (
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestNativeProductJourney(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	newVersion, err := readiness.ReadProductVersion(root)
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
		if runtime.GOOS == "linux" {
			journey.setEnvironment(
				"DBUS_SESSION_BUS_ADDRESS",
				"unix:path="+filepath.Join(journey.root, "missing-session-bus.sock"),
			)
		}
		journey.run("setup", "--from", journey.manifest)
		journey.requireConfigContains("native-system-keyring-probe-claude", "unused-claude")
		journey.requireNoClaudeProjection()

		journey.installClientFixture("claude")
		journey.run("sync")
		journey.requireNoClaudeProjection()
		journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
		preview := journey.run("sync", "--dry-run", "--json")
		if !json.Valid(preview) {
			t.Fatalf("sync preview is not JSON: %s", preview)
		}
		journey.run("sync")
		journey.requireClaudeProjection()
		journey.requireCredentialBackend("native-journey-token", secrets.BackendSelection{
			Kind:         "env",
			Availability: "available",
			Mutability:   "read_only",
			Persistence:  "explicit",
		})
		if got := journey.claudeCredential(); got != "native-journey-token" {
			t.Fatalf("credential = %q", got)
		}
		journey.run("check")
		journey.run("verify", "--for", "claude")
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

	t.Run("portable artifact lifecycle", func(t *testing.T) {
		runNativeReleaseLifecycle(t, root, artifact, newVersion, server.URL+"/v1")
	})

	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") != "1" {
		return
	}
	t.Run("system credential store", func(t *testing.T) {
		runNativeCredentialJourney(t, root, artifact, server.URL+"/v1", newVersion)
	})

	if runtime.GOOS == "linux" {
		t.Run("secure file fallback without session bus", func(t *testing.T) {
			journey := newNativeJourney(t, artifact, server.URL+"/v1", true)
			journey.enableSystemCredentialStore()
			journey.setEnvironment(
				"DBUS_SESSION_BUS_ADDRESS",
				"unix:path="+filepath.Join(journey.root, "missing-session-bus.sock"),
			)
			const token = "native-secure-file-token"
			journey.runWithInput(journey.binary, token+"\n", "setup", "--from", journey.manifest, "--account", "native-system-keyring-probe", "--token-stdin")
			journey.requireCredentialBackend(token, secrets.BackendSelection{
				Kind:         "file",
				Availability: "available",
				Mutability:   "read_write",
				Persistence:  "persisted",
			})
			if got := journey.claudeCredential(); got != token {
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

type journeyFixture struct {
	testing              *testing.T
	source               string
	binary               string
	root                 string
	clientBin            string
	manifest             string
	config               string
	settings             string
	endpoint             string
	environment          []string
	retainedProgramNames []string
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
		journey.installClientFixture("claude")
	}
	journey.runWith(source, "install", "--target", journey.binary)
	journey.run("--version")
	return journey
}

func (j *journeyFixture) installClientFixture(client string) {
	j.testing.Helper()
	name, content, mode := client, "#!/bin/sh\nprintf 'AIGW_OK\\n'\n", os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		name, content, mode = client+".cmd", "@echo off\r\necho AIGW_OK\r\n", 0o600
	}
	if err := os.WriteFile(filepath.Join(j.clientBin, name), []byte(content), mode); err != nil {
		j.testing.Fatal(err)
	}
}

func (j *journeyFixture) setEnvironment(key, value string) {
	j.testing.Helper()
	j.environment = append(environmentWithout(j.environment, key), key+"="+value)
}

func TestJourneyEnvironmentUpdatesPreserveOwnedCredentials(t *testing.T) {
	key := secrets.EnvironmentKey("native-system-keyring-probe")
	initial := environmentWith([]string{"PATH=host", key + "=ambient"}, map[string]string{"PATH": "isolated"})
	if !slices.Equal(initial, []string{"PATH=isolated"}) {
		t.Fatalf("initial environment retained ambient credentials: %v", initial)
	}
	journey := &journeyFixture{testing: t, environment: initial}
	journey.setEnvironment(key, "synthetic")
	journey.setEnvironment("PATH", "native-tools")
	journey.setEnvironment("PATH", "native-tools")
	if !slices.Equal(journey.environment, []string{key + "=synthetic", "PATH=native-tools"}) {
		t.Fatalf("environment update changed owned credentials: %v", journey.environment)
	}
}

func (j *journeyFixture) run(args ...string) []byte {
	j.testing.Helper()
	return j.runWithInput(j.binary, "", args...)
}

func (j *journeyFixture) runWith(binary string, args ...string) []byte {
	j.testing.Helper()
	return j.runWithInput(binary, "", args...)
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
	for _, path := range []string{j.settings, j.settings + ".aigw-state.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			j.testing.Fatalf("Claude projection unexpectedly exists at %s: %v", path, err)
		}
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

func (j *journeyFixture) claudeCredential() string {
	j.testing.Helper()
	var settings struct {
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil {
		j.testing.Fatal(err)
	}
	if settings.APIKeyHelper == "" {
		j.testing.Fatal("Claude projection lacks a credential helper")
	}
	if runtime.GOOS == "windows" {
		// Execute the exact helper as native shell source, not a quoted Go
		// argument: cmd.exe does not use CommandLineToArgvW escaping.
		script := filepath.Join(j.root, "credential helper.cmd")
		if err := os.WriteFile(script, []byte("@echo off\r\n"+settings.APIKeyHelper+"\r\n"), 0o600); err != nil {
			j.testing.Fatal(err)
		}
		output, err := (process.Runner{}).RunCapture(j.testing.Context(), process.Plan{Executable: script, Env: j.environment})
		if err != nil {
			j.testing.Fatalf("execute projected credential helper: %v", err)
		}
		return strings.TrimSpace(string(output))
	}
	return strings.TrimSpace(string(j.runWith("/bin/sh", "-c", settings.APIKeyHelper)))
}

func (j *journeyFixture) uninstallAndRequireOwnedFilesAbsent() {
	j.testing.Helper()
	j.runWith(j.source, "uninstall", "--target", j.binary)
	j.requireOwnedFilesAbsent()
	j.requireNoClaudeProjection()
	if _, err := os.Stat(j.config + ".verified.json"); !os.IsNotExist(err) {
		j.testing.Fatalf("uninstall retained verified checkpoint: %v", err)
	}
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
	if !slices.Equal(entryNames(entries), j.retainedProgramNames) {
		j.testing.Fatalf("uninstall retained lifecycle residue: %#v", entryNames(entries))
	}
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
