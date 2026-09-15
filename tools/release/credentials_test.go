package main

import (
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/keychain"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if handled, code := keychain.RunWorker(os.Args[1:], os.Stdin, os.Stdout, secrets.Service); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestReleaseCredentialWorkerObservesOnlyAnAbsentItem(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("native Keychain worker boundary")
	}
	account := "aigw-release-absent-" + filepath.Base(t.TempDir())
	if exists, err := keychain.Exists(secrets.Service, account); err != nil || exists {
		t.Fatalf("release credential worker observation: exists=%t error=%v", exists, err)
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
	baseline, err := nativeLifecycleBaseline(artifact)
	if err != nil {
		t.Fatal(err)
	}
	journey := newNativeJourney(t, baseline, endpoint, true)
	oldVersion := journey.predecessorVersion(newVersion)
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
	if got := journey.claudeCredential(); got != token {
		t.Fatalf("credential = %q", got)
	}
	journey.runWithInput(journey.binary, replacement+"\n", "rotate", "native-system-keyring-probe", "--token-stdin")
	if got := journey.claudeCredential(); got != replacement {
		t.Fatalf("rotated client credential = %q", got)
	}
	journey.requireStoredCredentialAcrossUpdate(root, newVersion, oldVersion, replacement, backend)
	journey.uninstallAndRequireOwnedFilesAbsent()
	if exists, err := store.Exists("native-system-keyring-probe"); err != nil || !exists {
		t.Fatalf("uninstall removed the retained credential: exists=%t error=%v", exists, err)
	}
	journey.runWith(journey.source, "install", "--target", journey.binary)
	journey.run("sync")
	journey.requireCredentialBackend(replacement, backend)
	if got := journey.claudeCredential(); got != replacement {
		t.Fatalf("reinstalled client lost the retained credential: %q", got)
	}
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
		j.run("sync")
		j.requireVersion(step.version)
		j.requireProgramBytes(step.program)
		if step.version == newVersion {
			j.requireCredentialBackend(token, backend)
		}
		if got := j.claudeCredential(); got != token {
			j.testing.Fatalf("program %s lost native credential continuity: %q", step.version, got)
		}
	}
	j.source = candidate
}
