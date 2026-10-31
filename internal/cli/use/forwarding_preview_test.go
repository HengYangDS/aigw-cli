package use

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	surfaceidentity "aigw-cli/internal/adapter/surface"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/secret"
)

// credentialFreeStore fails any credential read, so a preview that reads one
// fails instead of passing silently.
type credentialFreeStore struct{ secret.Store }

func (credentialFreeStore) Get(string) (string, error) {
	return "", errors.New("preview read a credential")
}

// A forwarding preview refuses a projected Codex target whose managed settings
// were edited outside AIGW, with the typed Codex conflict, and leaves the
// configuration, its recovery inputs and every file beside the target as they
// were.
func TestUseForwardingPreviewRefusesAnEditedOwnedTarget(t *testing.T) {
	run, cfg, out := configuredRuntime(t)
	target := filepath.Join(t.TempDir(), "config.toml")
	binding := cfg.Clients[codex.ID]
	binding.Enabled, binding.Targets = true, []string{target}
	binding.CredentialCommand = filepath.Join(t.TempDir(), "codex-reader")
	cfg.Clients[codex.ID] = binding
	if err := run.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, selected); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(projected, []byte("https://gateway.example.test/v1"), []byte("https://user.example.test/v1"), 1)
	if bytes.Equal(projected, edited) {
		t.Fatal("fixture did not change the managed endpoint")
	}
	if err := os.WriteFile(target, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := run.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	owned := directoryBytes(t, filepath.Dir(target))
	run.Secrets = credentialFreeStore{secret.NewMemoryStore()}
	command := NewCommand(run)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", codex.ID, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	err = command.Execute()
	if _, conflict := errors.AsType[*codex.ConflictError](err); !conflict {
		t.Fatalf("preview did not refuse the edited target as a Codex conflict: %v, %s", err, out.String())
	}
	after, err := run.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("refusal changed configuration or recovery inputs: %v", err)
	}
	if current := directoryBytes(t, filepath.Dir(target)); !reflect.DeepEqual(owned, current) {
		t.Fatal("refusal changed a file beside the edited target")
	}
}

// A forwarding preview plans a newly discovered Codex target without creating
// it or reading a credential, and the selection then writes the file that the
// preview planned.
func TestUseForwardingPreviewPlansANewlyDiscoveredTarget(t *testing.T) {
	run, cfg, out := configuredRuntime(t)
	run.Host.Home = t.TempDir()
	target := filepath.Join(run.Host.Home, "work", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	run.Discovery = staticDiscovery{result: discovery.Result{
		Executables: map[string]string{codex.ID: filepath.Join(t.TempDir(), "codex")},
		Surfaces: []discovery.Surface{{
			ID: string(codex.HomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
			ConfigPath: target, AutoManaged: true,
		}},
	}}
	binding := cfg.Clients[codex.ID]
	binding.Enabled, binding.CredentialCommand = true, filepath.Join(t.TempDir(), "reader")
	cfg.Clients[codex.ID] = binding
	if err := run.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	run.Secrets = credentialFreeStore{secret.NewMemoryStore()}
	command := NewCommand(run)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", codex.ID, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Projections []invocation.Projection `json:"projections"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	// The document names the target relative to the home directory.
	if len(preview.Projections) != 1 || preview.Projections[0].Target != "~/work/config.toml" ||
		preview.Projections[0].Client != codex.ID || preview.Projections[0].Action != "initial_project" {
		t.Fatalf("preview did not plan the newly discovered target: %s", out.Bytes())
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview created the discovered target: %v", err)
	}
	credentials := secret.NewMemoryStore()
	if err := credentials.Set("gateway", "synthetic"); err != nil {
		t.Fatal(err)
	}
	run.Secrets = credentials
	command = NewCommand(run)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", codex.ID, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	current, err := run.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	stored := current.Clients[codex.ID].Targets
	if len(stored) != 1 {
		t.Fatalf("selection stored %d targets: %#v", len(stored), stored)
	}
	// The binding keeps the discovered path, the one file that the preview
	// planned and the selection wrote.
	applied, appliedErr := os.Stat(stored[0])
	discovered, discoveredErr := os.Stat(target)
	if appliedErr != nil || discoveredErr != nil || !os.SameFile(applied, discovered) {
		t.Fatalf("selection did not apply the discovered target: %q, %q: %v, %v", stored[0], target, appliedErr, discoveredErr)
	}
}

func directoryBytes(t *testing.T, directory string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = data
	}
	return files
}
