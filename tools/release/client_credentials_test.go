package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

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
