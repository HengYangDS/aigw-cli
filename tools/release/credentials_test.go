package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	"bytes"
	"context"
	"crypto/sha256"
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
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "credential" && os.Getenv("AIGW_TEST_EXTERNAL_CREDENTIAL") == "1" {
		if os.Args[2] != os.Getenv("AIGW_TEST_EXTERNAL_CLIENT") || os.Args[3] != os.Getenv("AIGW_TEST_EXTERNAL_FINGERPRINT") {
			os.Exit(2)
		}
		_, _ = fmt.Fprintln(os.Stdout, "native-real-client-token")
		os.Exit(0)
	}
	os.Exit(m.Run())
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
	adapter := cfg.Adapters[client]
	adapter.CredentialCommand = helper
	cfg.Adapters[client] = adapter
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
	j.run("adapter", "disable", client)
	j.run("sync")
	args := []string{"adapter", "enable", client, "--executable", executable}
	if client == configuration.ClientCodex {
		args = append(args, "--target", filepath.Join(j.root, "home", ".codex", "config.toml"))
	}
	j.run(args...)
	j.run("sync")
	j.run("verify", "--for", client)
	retained, err := store.Load()
	if err != nil || retained.Adapters[client].CredentialCommand != helper {
		j.testing.Fatalf("native lifecycle discarded explicit helper: %v", err)
	}
	if completed() < count+2 {
		j.testing.Fatal("external helper did not authenticate both real-client invocations")
	}
}

func prepareNativeSigning(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" || os.Getenv("AIGW_MACOS_SIGNING_P12") != "" {
		return
	}
	root := t.TempDir()
	certificate := filepath.Join(root, "identity")
	p12, password, requirements := filepath.Join(root, "identity.p12"), filepath.Join(root, "password"), filepath.Join(root, "requirement.bin")
	if err := os.WriteFile(password, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(executable string, args ...string) string {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, executable, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("private signing fixture %s: %v\n%s", executable, err, output)
		}
		return string(output)
	}
	run("rcodesign", "-C", "/dev/null", "generate-self-signed-certificate", "--person-name", "aigw-native-journey", "--validity-days", "1", "--pem-filename", certificate, "--p12-file", p12, "--p12-password", "synthetic")
	fingerprint := run("/usr/bin/openssl", "x509", "-in", certificate+".crt", "-noout", "-fingerprint", "-sha1")
	_, fingerprint, found := strings.Cut(fingerprint, "=")
	if !found {
		t.Fatal("private signing certificate fingerprint absent")
	}
	fingerprint = strings.ReplaceAll(strings.TrimSpace(fingerprint), ":", "")
	run("/usr/bin/csreq", "-r", `=certificate leaf = H"`+fingerprint+`" and identifier "aigw"`, "-b", requirements)
	t.Setenv("AIGW_MACOS_SIGNING_P12", p12)
	t.Setenv("AIGW_MACOS_SIGNING_PASSWORD_FILE", password)
	t.Setenv("AIGW_MACOS_SIGNING_REQUIREMENTS", requirements)
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
		output := journey.runWithInput(journey.binary, input, "test", "--profile", "native-system-keyring-probe-claude", "--token-stdin", "--token-format", format, "--config", journey.config)
		if !bytes.Contains(output, []byte("not model inference")) || bytes.Contains(output, []byte(token)) {
			t.Fatal("endpoint result lost its evidence or secret boundary")
		}
	}
	if requests != 2 || !bytes.Equal(before, readFile(t, journey.config)) {
		t.Fatalf("ephemeral endpoint requests=%d or configuration changed", requests)
	}
	journey.requireNoClaudeProjection()
	journey.uninstallAndRequireOwnedFilesAbsent()
}

func runNativeCredentialJourney(t *testing.T, root, artifact, endpoint, newVersion string) {
	t.Helper()
	baseline := requireNativeLifecycleBaseline(t, func() string { return artifact })
	journey := newNativeJourney(t, baseline, endpoint, true)
	oldVersion := journey.predecessorVersion(newVersion)
	journey.prepareCodexLifecycle()
	journey.enableSystemCredentialStore()
	store, err := secrets.Select(secrets.Selection{Backend: "keyring"})
	if err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists("native-system-keyring-probe"); err != nil || exists {
		t.Fatalf("native credential test requires an unoccupied slot: exists=%t error=%v", exists, err)
	}
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
	journey.runWithInput(journey.binary, token+"\n", "setup", "--from", journey.manifest, "--account", "native-system-keyring-probe", "--token-stdin")
	journey.requireClaudeCredential(token)
	journey.runWithInput(journey.binary, replacement+"\n", "rotate", "native-system-keyring-probe", "--token-stdin")
	journey.requireClaudeCredential(replacement)
	journey.requireStoredCredentialAcrossUpdate(root, newVersion, oldVersion, replacement, backend)
	journey.uninstallAndRequireOwnedFilesAbsent()
	if exists, err := store.Exists("native-system-keyring-probe"); err != nil || !exists {
		t.Fatalf("uninstall removed the retained credential: exists=%t error=%v", exists, err)
	}
	journey.runWith(journey.source, "install", "--target", journey.binary)
	journey.run("sync")
	journey.requireCredentialBackend(replacement, backend)
	journey.requireClaudeCredential(replacement)
	journey.uninstallAndRequireOwnedFilesAbsent()
	if err := store.Delete("native-system-keyring-probe"); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists("native-system-keyring-probe"); err != nil || exists {
		t.Fatalf("deleted credential remains: exists=%t error=%v", exists, err)
	}
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

func TestSystemCredentialJourneyUsesItsEffectiveHome(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "ephemeral-host")
	root := t.TempDir()
	journey := &journeyFixture{
		testing:  t,
		config:   filepath.Join(root, "config", "aigw", "config.toml"),
		settings: filepath.Join(root, "home", ".claude", "settings.json"),
		environment: []string{
			"HOME=" + filepath.Join(root, "home"),
			"USERPROFILE=" + filepath.Join(root, "home"),
			"XDG_CONFIG_HOME=" + filepath.Join(root, "config"),
			"XDG_DATA_HOME=" + filepath.Join(root, "data"),
			"APPDATA=" + filepath.Join(root, "config"),
			"LOCALAPPDATA=" + filepath.Join(root, "data"),
			"AIGW_SECRET_BACKEND=env",
		},
	}
	wantHome := filepath.Join(root, "home")
	wantConfig := filepath.Join(root, "config", "aigw", "config.toml")
	if runtime.GOOS == "darwin" {
		wantHome = hostHome
		wantConfig = filepath.Join(hostHome, "Library", "Application Support", "aigw", "config.toml")
	}
	for range 2 {
		t.Run("isolated native paths", func(t *testing.T) {
			journey.testing = t
			journey.enableSystemCredentialStore()
			if journey.settings != filepath.Join(wantHome, ".claude", "settings.json") || journey.config != wantConfig {
				t.Fatalf("credential journey paths = %q, %q; want effective home %q and config %q", journey.settings, journey.config, wantHome, wantConfig)
			}
			if runtime.GOOS == "darwin" {
				for _, path := range []string{journey.config, journey.settings} {
					if err := os.WriteFile(path, []byte("test-owned state"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
		if runtime.GOOS == "darwin" {
			for _, path := range []string{journey.config, journey.settings} {
				if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
					t.Fatalf("native test directory remains after cleanup: %s, %v", path, err)
				}
			}
		}
	}
}

func (j *journeyFixture) enableSystemCredentialStore() {
	j.testing.Helper()
	environment, err := systemCredentialEnvironment(runtime.GOOS, j.environment, os.Getenv("HOME"), os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"))
	if err != nil {
		j.testing.Fatal(err)
	}
	j.environment = environment
	paths, err := platform.PathsFor(runtime.GOOS, environmentValues(environment))
	if err != nil {
		j.testing.Fatal(err)
	}
	j.config = filepath.FromSlash(paths.Config)
	j.settings = filepath.FromSlash(paths.ClaudeSettings)
	if runtime.GOOS == "darwin" {
		for _, directory := range []string{filepath.Dir(j.config), filepath.Dir(j.settings)} {
			if err := os.MkdirAll(filepath.Dir(directory), 0o700); err != nil {
				j.testing.Fatal(err)
			}
			if err := os.Mkdir(directory, 0o700); err != nil {
				j.testing.Fatalf("native credential test requires an unoccupied directory %s: %v", directory, err)
			}
			j.testing.Cleanup(func() {
				if err := os.RemoveAll(directory); err != nil {
					j.testing.Errorf("clean owned credential test directory %s: %v", directory, err)
				}
			})
		}
	}
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

func (j *journeyFixture) requireStoredCredentialAcrossUpdate(root, newVersion, oldVersion, token string, backend secrets.BackendSelection) {
	j.testing.Helper()
	candidate, archive, checksums := nativeReleaseCandidate(j.testing, root, newVersion)
	j.testing.Logf("credential baseline version=%s sha256=%x; candidate version=%s sha256=%x", oldVersion, sha256.Sum256(readFile(j.testing, j.source)), newVersion, sha256.Sum256(readFile(j.testing, candidate)))
	retained := j.retainedCredentials()
	for _, step := range []struct {
		version string
		program string
		args    []string
	}{
		{newVersion, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
		{oldVersion, j.source, []string{"update", "--rollback"}},
		{newVersion, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
	} {
		configurationBefore := readFile(j.testing, j.config)
		j.run(step.args...)
		if !bytes.Equal(readFile(j.testing, j.config), configurationBefore) {
			j.testing.Fatal("credential lifecycle changed the retained client configuration")
		}
		for _, credential := range retained {
			j.requireCredential(credential, token)
		}
		j.run("sync")
		j.requireVersion(step.version)
		j.requireProgramBytes(step.program)
		if step.version == newVersion {
			j.requireCredentialBackend(token, backend)
		}
		j.requireClaudeCredential(token)
	}
	j.source = candidate
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

func (j *journeyFixture) retainedCredential(client string) process.Plan {
	j.testing.Helper()
	plan := process.Plan{Env: slices.Clone(j.environment)}
	if client == configuration.ClientCodex {
		var config struct {
			ModelProvider  string `toml:"model_provider"`
			ModelProviders map[string]struct {
				Auth struct {
					Command string   `toml:"command"`
					Args    []string `toml:"args"`
				} `toml:"auth"`
			} `toml:"model_providers"`
		}
		path := filepath.Join(j.root, "home", ".codex", "config.toml")
		if err := toml.Unmarshal(readFile(j.testing, path), &config); err != nil {
			j.testing.Fatal(err)
		}
		auth := config.ModelProviders[config.ModelProvider].Auth
		if auth.Command == "" || len(auth.Args) == 0 {
			j.testing.Fatal("Codex projection lacks a credential command")
		}
		plan.Executable, plan.Args = auth.Command, auth.Args
		return plan
	}
	if client != configuration.ClientClaude {
		j.testing.Fatalf("unsupported credential client %q", client)
	}
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
		script, err := os.CreateTemp(j.root, "credential-*.cmd")
		if err != nil {
			j.testing.Fatal(err)
		}
		_, writeErr := script.WriteString("@echo off\r\n" + settings.APIKeyHelper + "\r\n")
		closeErr := script.Close()
		if writeErr != nil || closeErr != nil {
			j.testing.Fatalf("retain exact Windows credential command: write=%v close=%v", writeErr, closeErr)
		}
		plan.Executable = script.Name()
	} else {
		plan.Executable, plan.Args = "/bin/sh", []string{"-c", settings.APIKeyHelper}
	}
	return plan
}

func (j *journeyFixture) retainedCredentials() []process.Plan {
	clients := configuration.AdmittedClientIDs()
	credentials := make([]process.Plan, 0, len(clients))
	for _, client := range clients {
		credentials = append(credentials, j.retainedCredential(client))
	}
	return credentials
}
