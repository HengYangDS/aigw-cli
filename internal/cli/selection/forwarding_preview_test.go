package selection

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

// credentialFreeStore fails any credential read, so a preview that reads one
// fails instead of passing silently.
type credentialFreeStore struct{ secrets.Store }

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
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Enabled, binding.Targets = true, []string{target}
	binding.CredentialCommand = filepath.Join(t.TempDir(), "codex-reader")
	cfg.Clients[configuration.ClientCodex] = binding
	if err := run.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
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
	run.Secrets = credentialFreeStore{secrets.NewMemoryStore()}
	command := NewUseCommand(run)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
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
	target := filepath.Join(t.TempDir(), "config.toml")
	run.Discovery = staticDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: filepath.Join(t.TempDir(), "codex")},
		Surfaces: []discovery.Surface{{
			ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
			ConfigPath: target, AutoManaged: true,
		}},
	}}
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Enabled, binding.CredentialCommand = true, filepath.Join(t.TempDir(), "reader")
	cfg.Clients[configuration.ClientCodex] = binding
	if err := run.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	run.Secrets = credentialFreeStore{secrets.NewMemoryStore()}
	command := NewUseCommand(run)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Targets []client.ProjectionPlan `json:"targets"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	// The plan names the target's real path; the temporary directory itself
	// may sit behind a symbolic link, such as /var on macOS.
	directory, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	planned := filepath.Join(directory, filepath.Base(target))
	if len(preview.Targets) != 1 || preview.Targets[0].Target != planned ||
		preview.Targets[0].Client != configuration.ClientCodex || preview.Targets[0].Action != "initial-project" {
		t.Fatalf("preview did not plan the newly discovered target: %s", out.Bytes())
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview created the discovered target: %v", err)
	}
	credentials := secrets.NewMemoryStore()
	if err := credentials.Set("gateway", "synthetic"); err != nil {
		t.Fatal(err)
	}
	run.Secrets = credentials
	command = NewUseCommand(run)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	current, err := run.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	stored := current.Clients[configuration.ClientCodex].Targets
	if len(stored) != 1 {
		t.Fatalf("selection stored %d targets: %#v", len(stored), stored)
	}
	// The binding keeps the discovered path and the plan names its real path;
	// both must be the one file that the selection wrote.
	applied, appliedErr := os.Stat(stored[0])
	previewed, previewedErr := os.Stat(planned)
	if appliedErr != nil || previewedErr != nil || !os.SameFile(applied, previewed) {
		t.Fatalf("selection did not apply the target that its preview planned: %q, %q: %v, %v", stored[0], planned, appliedErr, previewedErr)
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
