package synchronization

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

// selfCompensationAdapter writes a client configuration that references the
// reader, then fails and cannot undo its own write, as a client adapter does
// when its sidecar write and its guarded self-restore both fail.
type selfCompensationAdapter struct {
	removedEntrypointAdapter
}

func (adapter selfCompensationAdapter) Apply(context.Context, client.Dependencies, domain.Config, domain.Config) (client.ProjectionReceipt, error) {
	if err := os.WriteFile(adapter.target, []byte("reader="+adapter.reader), 0o600); err != nil {
		return nil, err
	}
	return nil, store.CompensationFailed(errors.New("write projection state"), errors.New("restore projection: newer edit"))
}

func TestFailedSelfCompensationPreservesTheReaderItProjected(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "aigw"), filepath.Join(root, "codex.toml")
	for path, data := range map[string]string{source: "executable fixture", target: "user settings"} {
		if err := os.WriteFile(path, []byte(data), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	adapter := selfCompensationAdapter{removedEntrypointAdapter{target: target, reader: reader}}
	registry, err := client.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	configStore := &configStoreStub{}
	syncer := Synchronizer{Config: configStore, Registry: registry, AIGWExecutable: source, CredentialPath: reader}
	err = syncer.CommitProjection(t.Context(), domain.NewConfig(clients.Admission()), testConfig(target), "sync")
	if !errors.Is(err, client.ErrProjectionRollbackFailed) || !errors.Is(err, store.ErrCompensationIncomplete) {
		t.Fatalf("incomplete self-compensation was not reported as a failed rollback: %v", err)
	}
	if _, statErr := os.Stat(reader); statErr != nil {
		t.Fatalf("a reader that the surviving client configuration references was removed: %v", statErr)
	}
	if data, readErr := os.ReadFile(target); readErr != nil || string(data) != "reader="+reader {
		t.Fatalf("client configuration = %q, %v", data, readErr)
	}
}
