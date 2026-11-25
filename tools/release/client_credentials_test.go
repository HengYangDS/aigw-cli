package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"

	"aigw-cli/internal/store"

	"aigw-cli/internal/clients"
)

func TestNativeClientFilePreservation(t *testing.T) {
	t.Run("effective Hermes home", func(t *testing.T) {
		journey := journeyFixture{testing: t, root: t.TempDir()}
		home := t.TempDir()
		journey.setEnvironment("HERMES_HOME", home)
		check := journey.preserveClientFiles(hermes.ID)
		path := filepath.Join(home, "sessions", "user.jsonl")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preservation did not inspect the effective Hermes session path: %v", err)
		}
		if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := check(); err == nil {
			t.Fatal("effective Hermes session edits were not detected")
		}
	})
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
			check := journey.preserveClientFiles(codex.ID)
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
	for _, client := range []string{claude.ID, codex.ID} {
		t.Run(client, func(t *testing.T) {
			journey := newNativeJourney(t, program, server.URL, true)
			var projection string
			if client == codex.ID {
				projection, _ = journey.prepareCodexLifecycle()
			} else {
				projection = journey.settings
			}
			journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
			journey.run("setup", "--from", journey.manifest)
			retained := journey.retainedCredential(client)
			journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "changed-after-capture")
			if err := os.Remove(projection); err != nil {
				t.Fatal(err)
			}
			journey.requireCredential(retained, "native-journey-token")
		})
	}
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
	journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest)
	retained := journey.retainedCredential(claude.ID)
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
	journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest)
	retained := journey.retainedCredential(claude.ID)
	configStore := store.NewStore(journey.config, clients.Admission())
	before, err := configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.SetClientActivation(claude.ID, false, "", nil)
	if err := configStore.Save(after); err != nil {
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
	if err := configStore.Save(before); err != nil {
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
	journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest)
	configStore := store.NewStore(journey.config, clients.Admission())
	cfg, err := configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	executable := cfg.Clients[claude.ID].Executable
	helper := journey.credentialEntrypoint()
	retained := journey.retainedCredential(claude.ID)
	journey.run("client", "disable", claude.ID)
	for _, path := range []string{helper, helper + ".sha256"} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("disable removed a possible cached command %s: %v", path, err)
		}
	}
	journey.run("client", "enable", claude.ID, "--executable", executable)
	journey.requireCredential(retained, "native-journey-token")
	journey.requireCredential(journey.retainedCredential(claude.ID), "native-journey-token")
}

// versionedReaderPath is the versioned credential reader below dataDir for
// the program at source, as the program names its own.
func versionedReaderPath(dataDir, source, installName string) (string, error) {
	program, err := secret.ReadProgram(nil, source)
	if err != nil {
		return "", err
	}
	return program.EntrypointPath(dataDir, installName)
}
