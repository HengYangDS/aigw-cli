package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/platform/process"

	"aigw-cli/internal/store"

	"aigw-cli/internal/clients"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

func (j *journeyFixture) retainedCredential(client string) process.Plan {
	j.testing.Helper()
	plan := process.Plan{Env: slices.Clone(j.environment)}
	if client == codex.ID {
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
	if client == hermes.ID {
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
	if client == claudedesktop.ID {
		library, err := claudedesktop.LibraryPathFor(runtime.GOOS, environmentValues(j.environment))
		if err != nil {
			j.testing.Fatal(err)
		}
		profile := claudedesktop.PathsForLibrary(filepath.FromSlash(library)).Profile
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
	if client != claude.ID {
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
	cfg, err := store.NewStore(j.config, clients.Admission()).Load()
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
	case codex.ID:
		path, content = filepath.Join(home, ".codex", "sessions", "user.jsonl"), "{\"owner\":\"user\"}\n"
	case hermes.ID:
		path, content = filepath.Join(environmentValues(j.environment)["HERMES_HOME"], "sessions", "user.jsonl"), "{\"owner\":\"user\"}\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		j.testing.Fatal(err)
	}
	files := map[string][]byte{path: []byte(content)}
	if client == codex.ID {
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
