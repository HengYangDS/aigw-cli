package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

	"github.com/spf13/cobra"
)

func executeManifestCommand(command *cobra.Command) error {
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

// TestCommandIsAGroup holds bare `aigw config` to the other command groups,
// which show their help: the group itself runs nothing.
func TestCommandIsAGroup(t *testing.T) {
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Out: io.Discard})
	if command.Runnable() {
		t.Fatal("the config group runs a command of its own")
	}
}

func TestConfigurationCommandTreeMatchesItsManifestResponsibility(t *testing.T) {
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Out: io.Discard})
	var names []string
	for _, child := range command.Commands() {
		names = append(names, child.Name())
		if child.RunE == nil || child.Args == nil {
			t.Errorf("configuration operation %q lacks execution or argument admission", child.Name())
		}
	}
	if want := []string{"export", "path", "rollback"}; !slices.Equal(names, want) {
		t.Fatalf("configuration operations = %q, want %q", names, want)
	}
}

func TestPathPrintsTheConfiguredStorePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	out := &bytes.Buffer{}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: store.NewStore(path, clients.Admission()), Out: out})
	command.SetArgs([]string{"path"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != path {
		t.Fatalf("path output = %q, want %q", got, path)
	}
	out.Reset()
	command.SetArgs([]string{"path", "--json"})
	var document struct {
		SchemaVersion int    `json:"schema_version"`
		Path          string `json:"path"`
	}
	if err := executeManifestCommand(command); err != nil || json.Unmarshal(out.Bytes(), &document) != nil || document.Path != path || document.SchemaVersion != pathSchemaVersion {
		t.Fatalf("path document = %v, %s", err, out)
	}
}

func TestPathReturnsOutputFailure(t *testing.T) {
	command := newPathCommand(invocation.Context{Clients: clients.Registry(), Config: store.NewStore("configuration.toml", clients.Admission()), Out: failingWriter{}})
	if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), "write refused") {
		t.Fatalf("error = %v", err)
	}
}

func TestExportWritesASecretFreeRoundTripManifest(t *testing.T) {
	runtime, _, out := savedRuntime(t, localConfig())
	secretStore := runtime.Secrets
	if err := secretStore.Set("local", "must-not-appear"); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SetArgs([]string{"export"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
	if bytes.Contains(data, []byte("must-not-appear")) {
		t.Fatal("export leaked a system secret")
	}
	parsed, err := store.Parse(data, clients.Admission())
	if err != nil {
		t.Fatalf("parse exported manifest: %v\n%s", err, data)
	}
	// The local Codex binding stays local; the manifest carries Accounts and Routes.
	if len(parsed.Recommendations) != 0 || parsed.Models["local"].Provider != "local" {
		t.Fatalf("exported manifest = %#v", parsed)
	}
}

func TestExportSurfacesLoadAndOutputFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := newExportCommand(invocation.Context{Clients: clients.Registry(), Config: store.NewStore(path, clients.Admission()), Out: io.Discard})
		if err := executeManifestCommand(command); err == nil {
			t.Fatal("expected malformed configuration to fail")
		}
	})

	t.Run("output", func(t *testing.T) {
		runtime, _, _ := savedRuntime(t, localConfig())
		runtime.Out = failingWriter{}
		if err := executeManifestCommand(newExportCommand(runtime)); err == nil || !strings.Contains(err.Error(), "write refused") {
			t.Fatalf("error = %v", err)
		}
	})
}

// A configuration without models, here the empty one of a new home, exports
// a manifest that the import accepts.
func TestExportWritesAManifestForAConfigurationWithoutModels(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "missing.toml"), clients.Admission())
	out := &bytes.Buffer{}
	command := newExportCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Out: out})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	if manifest, err := store.Parse(out.Bytes(), clients.Admission()); err != nil || len(manifest.Models) != 0 {
		t.Fatalf("exported manifest %q = %+v, %v", out, manifest, err)
	}
}

func savedRuntime(t *testing.T, cfg domain.Config) (invocation.Context, string, *bytes.Buffer) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(path, clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return invocation.Context{Clients: clients.Registry(),
		Config:  configStore,
		Secrets: secret.NewMemoryStore(),
		Out:     out,
		Width:   120,
	}, path, out
}

func localConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["local"] = domain.Provider{Label: "Local", Endpoints: domain.Endpoints{OpenAIResponses: "https://local.example/v1"}}
	cfg.Models["local"] = domain.Model{
		Label: "Local", Provider: "local", Family: "gpt-local",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedModel(codex.ID, "local", "")
	return cfg
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write refused") }
