package manifest

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

const importManifest = `version = 3
recommended_default = "remote"

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
openai_responses = "https://gateway.example/v1"

[profiles.remote]
label = "Remote"
account = "gateway"
client = "codex"

[profiles.remote.models]
codex = "gpt-remote"
`

func executeManifestCommand(command *cobra.Command) error {
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

func TestCommandRequiresAConfigurationOperation(t *testing.T) {
	command := NewCommand(invocation.Context{Out: io.Discard})
	if err := command.RunE(command, nil); err == nil || !strings.Contains(err.Error(), "Choose a config subcommand") {
		t.Fatalf("missing-operation error = %v", err)
	}
	if err := command.RunE(command, []string{"unknown"}); err == nil || !strings.Contains(err.Error(), `Unknown config subcommand "unknown"`) {
		t.Fatalf("unknown-operation error = %v", err)
	}
}

func TestPathPrintsTheConfiguredStorePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	out := &bytes.Buffer{}
	command := NewCommand(invocation.Context{Config: configuration.NewStore(path), Out: out})
	command.SetArgs([]string{"path"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != path {
		t.Fatalf("path output = %q, want %q", got, path)
	}
}

func TestPathReturnsOutputFailure(t *testing.T) {
	command := newPathCommand(invocation.Context{Config: configuration.NewStore("configuration.toml"), Out: failingWriter{}})
	if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), "write refused") {
		t.Fatalf("error = %v", err)
	}
}

func TestExportWritesASecretFreeRoundTripManifest(t *testing.T) {
	runtime, _ := savedRuntime(t, localConfig())
	secretStore := runtime.Secrets.(*secrets.MemoryStore)
	if err := secretStore.Set("local", "must-not-appear"); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SetArgs([]string{"export"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	data := runtime.Out.(*bytes.Buffer).Bytes()
	if bytes.Contains(data, []byte("must-not-appear")) {
		t.Fatal("export leaked a system secret")
	}
	parsed, err := configuration.Parse(data)
	if err != nil {
		t.Fatalf("parse exported manifest: %v\n%s", err, data)
	}
	if parsed.RecommendedDefault != "local" || parsed.Profiles["local"].Account != "local" {
		t.Fatalf("exported manifest = %#v", parsed)
	}
}

func TestExportSurfacesLoadAndOutputFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := newExportCommand(invocation.Context{Config: configuration.NewStore(path), Out: io.Discard})
		if err := executeManifestCommand(command); err == nil {
			t.Fatal("expected malformed configuration to fail")
		}
	})

	t.Run("output", func(t *testing.T) {
		runtime, _ := savedRuntime(t, localConfig())
		runtime.Out = failingWriter{}
		if err := executeManifestCommand(newExportCommand(runtime)); err == nil || !strings.Contains(err.Error(), "write refused") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestExportSurfacesManifestValidationFailure(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "missing.toml"))
	command := newExportCommand(invocation.Context{Config: store, Out: io.Discard})
	if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), "at least one profile") {
		t.Fatalf("error = %v", err)
	}
}

func TestImportMergesConfigurationAndReportsOneMissingToken(t *testing.T) {
	runtime, path := savedRuntime(t, localConfig())
	manifestPath := writeManifest(t, importManifest)
	command := NewCommand(runtime)
	command.SetArgs([]string{"import", manifestPath})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}

	loaded, err := configuration.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profiles["remote"].Account != "gateway" {
		t.Fatalf("imported config = %#v", loaded)
	}
	output := runtime.RenderOut.(*bytes.Buffer).String()
	for _, want := range []string{"Configuration manifest imported", "Profiles", "Accounts", "Token required", "aigw rotate gateway"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
}

func TestImportSelectsNextStepFromCredentialAvailability(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		seed     []string
		want     string
	}{
		{name: "all available", manifest: importManifest, seed: []string{"gateway"}, want: "aigw models"},
		{name: "multiple missing", manifest: twoAccountManifest(), want: "aigw rotate <account>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime, _ := savedRuntime(t, localConfig())
			for _, account := range test.seed {
				if err := runtime.Secrets.Set(account, "token"); err != nil {
					t.Fatal(err)
				}
			}
			command := newImportCommand(runtime)
			command.SetArgs([]string{writeManifest(t, test.manifest)})
			if err := executeManifestCommand(command); err != nil {
				t.Fatal(err)
			}
			if output := runtime.RenderOut.(*bytes.Buffer).String(); !strings.Contains(output, test.want) {
				t.Fatalf("output %q does not contain %q", output, test.want)
			}
		})
	}
}

func TestImportReplacementFlagsMakeIdentityChangesExplicit(t *testing.T) {
	conflicting := strings.ReplaceAll(importManifest, "gateway", "local")
	conflicting = strings.ReplaceAll(conflicting, "remote", "local")

	runtime, path := savedRuntime(t, localConfig())
	manifestPath := writeManifest(t, conflicting)
	withoutConsent := newImportCommand(runtime)
	withoutConsent.SetArgs([]string{manifestPath})
	if err := executeManifestCommand(withoutConsent); err == nil || !strings.Contains(err.Error(), "conflicts with local configuration") {
		t.Fatalf("error = %v", err)
	}

	withConsent := newImportCommand(runtime)
	withConsent.SetArgs([]string{manifestPath, "--replace-account", "local", "--replace-profile", "local"})
	if err := executeManifestCommand(withConsent); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Accounts["local"].Label != "Gateway" || loaded.Profiles["local"].Label != "Remote" {
		t.Fatalf("explicit replacement did not converge: %#v", loaded)
	}
}

func TestRendererFallsBackToThePrimaryOutput(t *testing.T) {
	out := &bytes.Buffer{}
	renderer(invocation.Context{Out: out, Width: 120}).Row("Account", "gateway")
	if output := out.String(); !strings.Contains(output, "Account") || !strings.Contains(output, "gateway") {
		t.Fatalf("output = %q", output)
	}
}

func TestImportSurfacesReadParseLoadAndMergeFailures(t *testing.T) {
	validPath := writeManifest(t, importManifest)
	tests := []struct {
		name    string
		runtime func(*testing.T) invocation.Context
		path    func(*testing.T) string
		want    string
	}{
		{
			name: "read",
			runtime: func(t *testing.T) invocation.Context {
				runtime, _ := savedRuntime(t, localConfig())
				return runtime
			},
			path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent.toml") },
			want: "Failed to read configuration manifest",
		},
		{
			name: "parse",
			runtime: func(t *testing.T) invocation.Context {
				runtime, _ := savedRuntime(t, localConfig())
				return runtime
			},
			path: func(t *testing.T) string { return writeManifest(t, "not = [valid") },
			want: "parse configuration manifest",
		},
		{
			name: "load",
			runtime: func(t *testing.T) invocation.Context {
				path := filepath.Join(t.TempDir(), "configuration.toml")
				if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
					t.Fatal(err)
				}
				return invocation.Context{Config: configuration.NewStore(path), Secrets: secrets.NewMemoryStore(), Out: io.Discard}
			},
			path: func(*testing.T) string { return validPath },
			want: "parse config",
		},
		{
			name: "merge",
			runtime: func(t *testing.T) invocation.Context {
				runtime, _ := savedRuntime(t, localConfig())
				return runtime
			},
			path: func(t *testing.T) string {
				conflicting := strings.ReplaceAll(importManifest, "gateway", "local")
				return writeManifest(t, conflicting)
			},
			want: "conflicts with local configuration",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := newImportCommand(test.runtime(t))
			command.SetArgs([]string{test.path(t)})
			if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func savedRuntime(t *testing.T, cfg configuration.Config) (invocation.Context, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	renderOut := &bytes.Buffer{}
	return invocation.Context{
		Config:    store,
		Secrets:   secrets.NewMemoryStore(),
		Out:       out,
		RenderOut: renderOut,
		Width:     120,
	}, path
}

func localConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["local"] = configuration.Account{Label: "Local", Endpoints: configuration.Endpoints{OpenAIResponses: "https://local.example/v1"}}
	cfg.Profiles["local"] = configuration.Profile{Label: "Local", Account: "local", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-local"}}
	cfg.Routes.Default = "local"
	return cfg
}

func writeManifest(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func twoAccountManifest() string {
	return strings.Replace(importManifest, "[profiles.remote]", `[accounts.backup]
label = "Backup"

[accounts.backup.endpoints]
openai_responses = "https://backup.example/v1"

[profiles.backup]
label = "Backup"
account = "backup"
client = "codex"

[profiles.backup.models]
codex = "gpt-backup"

[profiles.remote]`, 1)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write refused") }
