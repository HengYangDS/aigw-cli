package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"bytes"
	"encoding/base64"
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

func TestMain(m *testing.M) {
	if handled, code := runInstalledClientFixture(os.Args[0], os.Args[1:]); handled {
		os.Exit(code)
	}
	if len(os.Args) == 4 && os.Args[1] == "credential" && os.Getenv("AIGW_TEST_EXTERNAL_CREDENTIAL") == "1" {
		if os.Args[2] != os.Getenv("AIGW_TEST_EXTERNAL_CLIENT") || os.Args[3] != os.Getenv("AIGW_TEST_EXTERNAL_FINGERPRINT") {
			os.Exit(2)
		}
		_, _ = fmt.Fprintln(os.Stdout, "native-real-client-token")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runInstalledClientFixture(executable string, args []string) (bool, int) {
	client := strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
	if client == configuration.ClientClaude {
		_, _ = fmt.Fprintln(os.Stdout, "AIGW_OK")
		return true, 0
	}
	if client != configuration.ClientCodex {
		return false, 0
	}
	if slices.Equal(args, []string{"--version"}) {
		_, _ = fmt.Fprintln(os.Stdout, "codex-cli 0.0.0-fixture")
		return true, 0
	}
	for index, argument := range args {
		if argument == "--output-last-message" && index+1 < len(args) {
			if err := os.WriteFile(args[index+1], []byte("AIGW_OK\n"), 0o600); err != nil {
				return true, 3
			}
			return true, 0
		}
	}
	return true, 2
}

func TestCodexFixtureWritesItsFinalResponse(t *testing.T) {
	fixture := &journeyFixture{testing: t, clientBin: t.TempDir()}
	fixture.installClientFixture(configuration.ClientCodex)
	executable := filepath.Join(fixture.clientBin, configuration.ClientCodex)
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if version, err := exec.Command(executable, "--version").Output(); err != nil || !strings.Contains(string(version), "codex-cli") {
		t.Fatalf("Codex fixture version = %q, %v", version, err)
	}
	response := filepath.Join(t.TempDir(), "response.txt")
	if output, err := exec.Command(executable, "exec", "--output-last-message", response).Output(); err != nil || len(output) != 0 {
		t.Fatalf("Codex fixture output = %q, %v", output, err)
	}
	if got := strings.TrimSpace(string(readFile(t, response))); got != "AIGW_OK" {
		t.Fatalf("Codex fixture final response = %q", got)
	}
}

func (j *journeyFixture) requireExternalCredentialClient(client, executable, account string, completed func() int64) {
	j.testing.Helper()
	count := completed()
	program, err := os.Executable()
	if err != nil {
		j.testing.Fatal(err)
	}
	helper := filepath.Join(j.root, "credential helper")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	if err := os.WriteFile(helper, readFile(j.testing, program), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	store := configuration.NewStore(j.config)
	cfg, err := store.Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	adapter := cfg.Clients[client]
	adapter.CredentialCommand = helper
	cfg.Clients[client] = adapter
	if err := store.Save(cfg); err != nil {
		j.testing.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(client, "")
	if err != nil {
		j.testing.Fatal(err)
	}
	j.environment = environmentWithout(j.environment, secrets.EnvironmentKey(account))
	j.setEnvironment("AIGW_TEST_EXTERNAL_CREDENTIAL", "1")
	j.setEnvironment("AIGW_TEST_EXTERNAL_CLIENT", client)
	j.setEnvironment("AIGW_TEST_EXTERNAL_FINGERPRINT", selected.CredentialProjectionFingerprint(client))
	before := readFile(j.testing, j.config)
	j.run("sync", "--dry-run", "--json")
	if !bytes.Equal(before, readFile(j.testing, j.config)) {
		j.testing.Fatal("external helper dry-run changed host configuration")
	}
	j.run("sync")
	j.run("check", "--json")
	j.run("verify", "--for", client)
	j.run("client", "disable", client)
	j.run("sync")
	args := []string{"client", "enable", client, "--executable", executable}
	if client == configuration.ClientCodex {
		args = append(args, "--target", filepath.Join(j.root, "home", ".codex", "config.toml"))
	}
	j.run(args...)
	j.run("sync")
	j.run("verify", "--for", client)
	retained, err := store.Load()
	if err != nil || retained.Clients[client].CredentialCommand != helper {
		j.testing.Fatalf("native lifecycle discarded explicit helper: %v", err)
	}
	if completed() < count+2 {
		j.testing.Fatal("external helper did not authenticate both real-client invocations")
	}
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

func runNativeEphemeralCredentials(t *testing.T, artifact string) {
	t.Helper()
	const token = "native-ephemeral-token"
	requests := 0
	endpoint := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("X-Api-Key") != token {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		response.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(endpoint.Close)
	journey := newNativeJourney(t, artifact, endpoint.URL, false)
	journey.run("setup", "--from", journey.manifest)
	before := readFile(t, journey.config)
	for _, format := range []string{"raw", "go-keyring-base64"} {
		input := token + "\r\n"
		if format == "go-keyring-base64" {
			input = "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))
		}
		output := journey.runWithInput(journey.binary, input, "test", "--for", "claude", "--route", "native-system-keyring-probe-claude", "--token-stdin", "--token-format", format, "--config", journey.config)
		if !bytes.Contains(output, []byte("not model inference")) || bytes.Contains(output, []byte(token)) {
			t.Fatal("endpoint result lost its evidence or secret boundary")
		}
	}
	if requests != 2 || !bytes.Equal(before, readFile(t, journey.config)) {
		t.Fatalf("ephemeral endpoint requests=%d or configuration changed", requests)
	}
	journey.requireNoClaudeProjection()
	journey.uninstallAndRequireInstallationRemoved()
}

func requiredClientInput(key string, directory bool) (string, error) {
	path := os.Getenv(key)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s requires an explicit absolute path", key)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return "", fmt.Errorf("%s has the wrong input kind", key)
	}
	return path, nil
}
