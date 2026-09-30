package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"time"

	claudedesktop "aigw-cli/internal/claude/desktop"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

func TestNativeClientFilePreservation(t *testing.T) {
	for _, test := range []struct {
		name string
		auth []byte
	}{
		{"absent authentication", nil},
		{"empty authentication", []byte{}},
		{"existing authentication", []byte("{\"owner\":\"user\"}\n")},
	} {
		t.Run(test.name, func(t *testing.T) {
			journey := journeyFixture{testing: t, root: t.TempDir()}
			home := filepath.Join(journey.root, "home", ".codex")
			if err := os.MkdirAll(home, 0o700); err != nil {
				t.Fatal(err)
			}
			if test.auth != nil {
				if err := os.WriteFile(filepath.Join(home, "auth.json"), test.auth, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			check := journey.preserveClientFiles(configuration.ClientCodex)
			if err := check(); err != nil {
				t.Fatal(err)
			}
			auth := filepath.Join(home, "auth.json")
			if err := os.WriteFile(auth, []byte("changed"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := check(); err == nil {
				t.Fatal("changed authentication was accepted")
			}
			if err := os.Remove(auth); err != nil {
				t.Fatal(err)
			}
			if err := check(); (err == nil) != (test.auth == nil) {
				t.Fatalf("authentication absence: initial=%q, error=%v", test.auth, err)
			}
			if err := os.Mkdir(auth, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := check(); err == nil {
				t.Fatal("a directory was accepted as an authentication file")
			}
		})
	}
}

func TestRetainedCredentialCommandDoesNotReloadClientProjection(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		t.Run(client, func(t *testing.T) {
			journey := newNativeJourney(t, program, server.URL, true)
			var projection string
			if client == configuration.ClientCodex {
				projection, _ = journey.prepareCodexLifecycle()
			} else {
				projection = journey.settings
			}
			journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
			journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
			retained := journey.retainedCredential(client)
			journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "changed-after-capture")
			if err := os.Remove(projection); err != nil {
				t.Fatal(err)
			}
			journey.requireCredential(retained, "native-journey-token")
		})
	}
}

func TestWindowsCredentialCommandSupportsDeepEntrypointNamespace(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows native shell and executable path contract")
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, namespace := range []struct {
		name   string
		prefix string
	}{{name: "drive"}, {name: "extended", prefix: `\\?\`}} {
		t.Run(namespace.name, func(t *testing.T) {
			root := t.TempDir()
			data := namespace.prefix + filepath.Join(root, strings.Repeat("deep", 32))
			if err := os.MkdirAll(data, 0o700); err != nil {
				t.Fatal(err)
			}
			reader, err := credential.VersionedEntrypointPath(data, program, "aigw.exe")
			if err != nil {
				t.Fatal(err)
			}
			if len(reader) < 260 {
				t.Fatal("fixture did not reach the native Windows shell path boundary")
			}
			command, err := credential.Command(reader, configuration.ClientClaude, "deep-fixture", runtime.GOOS)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := credential.EnsureEntrypoint(program, reader); err != nil {
				t.Fatal(err)
			}
			prepared, err := credential.Command(reader, configuration.ClientClaude, "deep-fixture", runtime.GOOS)
			if err != nil || command != prepared {
				t.Fatalf("reader preparation changed the captured command: %v", err)
			}
			canonical, err := credential.ExecutableFromCommand(command, configuration.ClientClaude, "deep-fixture", runtime.GOOS)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.Stat(canonical)
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(reader)
			if err != nil || !os.SameFile(actual, original) {
				t.Fatalf("native shell path changed the credential file identity: %v", err)
			}
			successor := filepath.Join(root, "successor.exe")
			mustWriteFile(t, successor, []byte("successor identity"), 0o700)
			current, err := credential.VersionedEntrypointPath(data, successor, "aigw.exe")
			if err != nil {
				t.Fatal(err)
			}
			if err := credential.ValidateRetainedEntrypoint(current, canonical); err != nil {
				t.Fatalf("same native reader namespace was rejected: %v", err)
			}
			journey := &journeyFixture{testing: t, root: root, environment: append(os.Environ(),
				"AIGW_TEST_EXTERNAL_CREDENTIAL=1", "AIGW_TEST_EXTERNAL_CLIENT=claude",
				"AIGW_TEST_EXTERNAL_FINGERPRINT=deep-fixture")}
			journey.requireCredential(journey.shellCredential(command), "native-real-client-token")
		})
	}
	t.Run("unavailable native name", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, strings.Repeat("uncreated", 32), "aigw.exe")
		if _, err := credential.Command(path, configuration.ClientClaude, "deep-fixture", runtime.GOOS); err == nil {
			t.Fatal("an overlong uncreated path acquired an executable command")
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("native path observation created a reader or alias: %v", err)
		}
	})
}

func TestRetainedCredentialSurvivesInstalledExecutableUnlink(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	journey := newNativeJourney(t, program, server.URL, true)
	journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
	retained := journey.retainedCredential(configuration.ClientClaude)
	journey.requireCredential(retained, "native-journey-token")
	if err := os.Remove(journey.binary); err != nil {
		t.Fatal(err)
	}
	var callers sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		callers.Go(func() {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			output, err := (process.Runner{}).RunCapture(ctx, retained)
			if err != nil {
				failures <- err
			} else if strings.TrimSpace(string(output)) != "native-journey-token" {
				failures <- errors.New("concurrent retained credential returned unexpected content")
			}
		})
	}
	callers.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
}

func TestSyncPreservesRetainedEntrypointAfterInterruptedLastClientWithdrawal(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	journey := newNativeJourney(t, program, server.URL, true)
	journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
	retained := journey.retainedCredential(configuration.ClientClaude)
	store := configuration.NewStore(journey.config)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.SetClientActivation(configuration.ClientClaude, false, "", nil)
	if err := store.Save(after); err != nil {
		t.Fatal(err)
	}
	helper := journey.credentialEntrypoint()
	beforePreview := readFile(t, journey.config)
	var preview struct {
		CredentialEntrypoint *struct {
			Path   string `json:"path"`
			Action string `json:"action"`
		} `json:"credential_entrypoint"`
	}
	if err := json.Unmarshal(journey.run("sync", "--dry-run", "--json"), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.CredentialEntrypoint != nil {
		t.Fatalf("dry-run proposed deleting a cached command: %#v", preview.CredentialEntrypoint)
	}
	if got := readFile(t, journey.config); !bytes.Equal(got, beforePreview) {
		t.Fatal("dry-run changed Client Bindings")
	}
	if _, err := os.Lstat(helper); err != nil {
		t.Fatalf("dry-run removed credential entrypoint: %v", err)
	}
	journey.run("sync")
	for _, path := range []string{helper, helper + ".sha256"} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("sync removed a possible cached command %s: %v", path, err)
		}
	}
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	journey.run("sync")
	journey.requireCredential(retained, "native-journey-token")
}

func TestClientDisablePreservesRetainedEntrypointAcrossReenable(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	journey := newNativeJourney(t, program, server.URL, true)
	journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
	store := configuration.NewStore(journey.config)
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	executable := cfg.Clients[configuration.ClientClaude].Executable
	helper := journey.credentialEntrypoint()
	retained := journey.retainedCredential(configuration.ClientClaude)
	journey.run("client", "disable", configuration.ClientClaude)
	for _, path := range []string{helper, helper + ".sha256"} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("disable removed a possible cached command %s: %v", path, err)
		}
	}
	journey.run("client", "enable", configuration.ClientClaude, "--executable", executable)
	journey.requireCredential(retained, "native-journey-token")
	journey.requireCredential(journey.retainedCredential(configuration.ClientClaude), "native-journey-token")
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
		path := filepath.Join(environmentValues(j.environment)["CODEX_HOME"], "config.toml")
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
	if client == configuration.ClientHermes {
		var config struct {
			Model struct {
				Provider string `yaml:"provider"`
			} `yaml:"model"`
			Providers map[string]struct {
				KeyCommand string `yaml:"key_cmd"`
			} `yaml:"providers"`
		}
		path := filepath.Join(environmentValues(j.environment)["HERMES_HOME"], "config.yaml")
		if err := yaml.Unmarshal(readFile(j.testing, path), &config); err != nil {
			j.testing.Fatal(err)
		}
		if config.Model.Provider == "" {
			j.testing.Fatal("Hermes projection lacks a selected provider")
		}
		command := config.Providers[config.Model.Provider].KeyCommand
		if command == "" {
			j.testing.Fatal("Hermes projection lacks a credential command")
		}
		return j.shellCredential(command)
	}
	if client == configuration.ClientClaudeDesktop {
		paths, err := platform.PathsFor(runtime.GOOS, environmentValues(j.environment))
		if err != nil {
			j.testing.Fatal(err)
		}
		profile := claudedesktop.PathsForLibrary(filepath.FromSlash(paths.ClaudeDesktopLibrary)).Profile
		var config struct {
			Command string   `json:"inferenceCredentialHelper"`
			Args    []string `json:"inferenceCredentialHelperArgs"`
		}
		if err := json.Unmarshal(readFile(j.testing, profile), &config); err != nil {
			j.testing.Fatal(err)
		}
		if config.Command == "" || len(config.Args) == 0 {
			j.testing.Fatal("Claude Desktop projection lacks a credential command")
		}
		plan.Executable, plan.Args = config.Command, config.Args
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
	return j.shellCredential(settings.APIKeyHelper)
}

func (j *journeyFixture) shellCredential(command string) process.Plan {
	plan := process.Plan{Env: slices.Clone(j.environment)}
	if runtime.GOOS == "windows" {
		// Execute the exact helper as native shell source, not a quoted Go
		// argument: cmd.exe does not use CommandLineToArgvW escaping.
		script, err := os.CreateTemp(j.root, "credential-*.cmd")
		if err != nil {
			j.testing.Fatal(err)
		}
		_, writeErr := script.WriteString("@echo off\r\n" + command + "\r\n")
		closeErr := script.Close()
		if writeErr != nil || closeErr != nil {
			j.testing.Fatalf("retain exact Windows credential command: write=%v close=%v", writeErr, closeErr)
		}
		plan.Executable = script.Name()
	} else {
		plan.Executable, plan.Args = "/bin/sh", []string{"-c", command}
	}
	return plan
}

func (j *journeyFixture) retainedCredentials() []process.Plan {
	cfg, err := configuration.NewStore(j.config).Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	credentials := make([]process.Plan, 0, len(cfg.Clients))
	for _, client := range cfg.EnabledClientIDs() {
		credentials = append(credentials, j.retainedCredential(client))
	}
	return credentials
}

type credentialReaderSnapshot struct {
	paths    [2]string
	contents [2][]byte
	exists   [2]bool
}

func (j *journeyFixture) captureCredentialReader() credentialReaderSnapshot {
	j.testing.Helper()
	reader := j.credentialEntrypoint()
	snapshot := credentialReaderSnapshot{paths: [2]string{reader, reader + ".sha256"}}
	for index, path := range snapshot.paths {
		contents, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			j.testing.Fatalf("inspect credential reader %s: %v", path, err)
		}
		snapshot.contents[index] = contents
		snapshot.exists[index] = err == nil
	}
	return snapshot
}

func (snapshot credentialReaderSnapshot) requireUnchanged(t *testing.T) {
	t.Helper()
	for index, path := range snapshot.paths {
		contents, err := os.ReadFile(path)
		if snapshot.exists[index] {
			if err != nil || !bytes.Equal(contents, snapshot.contents[index]) {
				t.Fatalf("uninstall changed retained credential reader %s: %v", path, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("uninstall created an absent credential reader %s: %v", path, err)
		}
	}
}

func (j *journeyFixture) requireWithdrawnCredentialDenied(plan process.Plan, token string) {
	j.testing.Helper()
	ctx, cancel := context.WithTimeout(j.testing.Context(), 10*time.Second)
	defer cancel()
	output, err := (process.Runner{}).RunCapture(ctx, plan)
	if err == nil || !bytes.Contains(output, []byte("adapter is not enabled")) || bytes.Contains(output, []byte(token)) {
		j.testing.Fatal("uninstall did not revoke the captured client's credential authorization")
	}
}

func (j *journeyFixture) preserveClientFiles(client string) func() error {
	j.testing.Helper()
	home := filepath.Join(j.root, "home")
	path, content := filepath.Join(home, ".claude", "CLAUDE.md"), "# User instructions\n"
	switch client {
	case configuration.ClientCodex:
		path, content = filepath.Join(home, ".codex", "sessions", "user.jsonl"), "{\"owner\":\"user\"}\n"
	case configuration.ClientHermes:
		path, content = filepath.Join(home, ".hermes", "sessions", "user.jsonl"), "{\"owner\":\"user\"}\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		j.testing.Fatal(err)
	}
	files := map[string][]byte{path: []byte(content)}
	if client == configuration.ClientCodex {
		auth := filepath.Join(home, ".codex", "auth.json")
		data, err := os.ReadFile(auth)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			j.testing.Fatal(err)
		}
		files[auth] = data // nil records absence; an empty file has non-nil bytes.
	}
	return func() error {
		for path, want := range files {
			got, err := os.ReadFile(path)
			if want == nil && errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil || want == nil || !bytes.Equal(got, want) {
				return errors.Join(fmt.Errorf("client lifecycle changed user file %s", path), err)
			}
		}
		return nil
	}
}
