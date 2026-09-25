package main

import (
	"context"
	"encoding/json"
	"errors"
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

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

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
