package synchronization

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
)

// selfCompensationAdapter writes a client configuration that references the
// reader, then fails and cannot undo its own write, as a client adapter does
// when its sidecar write and its guarded self-restore both fail.
type selfCompensationAdapter struct {
	removedEntrypointAdapter
}

func (clientAdapter selfCompensationAdapter) Apply(context.Context, adapter.Dependencies, domain.Config, domain.Config) (adapter.ProjectionReceipt, error) {
	if err := os.WriteFile(clientAdapter.target, []byte("reader="+clientAdapter.reader), 0o600); err != nil {
		return nil, err
	}
	return nil, platform.CompensationFailed(errors.New("write projection state"), errors.New("restore projection: newer edit"))
}

func TestFailedSelfCompensationPreservesTheReaderItProjected(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "aigw"), filepath.Join(root, "codex.toml")
	for path, data := range map[string]string{source: "executable fixture", target: "user settings"} {
		if err := os.WriteFile(path, []byte(data), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	reader, err := secret.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	clientAdapter := selfCompensationAdapter{removedEntrypointAdapter{target: target, reader: reader}}
	registry, err := adapter.NewRegistry(clientAdapter)
	if err != nil {
		t.Fatal(err)
	}
	configStore := &configStoreStub{}
	syncer := Synchronizer{Config: configStore, Registry: registry, AIGWExecutable: source, ResolveCredentialPath: readerAt(reader)}
	err = syncer.CommitProjection(t.Context(), domain.NewConfig(clients.Admission()), testConfig(target), "sync")
	if !errors.Is(err, adapter.ErrProjectionRollbackFailed) || !errors.Is(err, platform.ErrCompensationIncomplete) {
		t.Fatalf("incomplete self-compensation was not reported as a failed rollback: %v", err)
	}
	if _, statErr := os.Stat(reader); statErr != nil {
		t.Fatalf("a reader that the surviving client configuration references was removed: %v", statErr)
	}
	if data, readErr := os.ReadFile(target); readErr != nil || string(data) != "reader="+reader {
		t.Fatalf("client configuration = %q, %v", data, readErr)
	}
}

// undoRefusingStore stores Tokens but refuses to delete one, as a credential
// store that locked after a Token was written does.
type undoRefusingStore struct{ secret.Store }

func (undoRefusingStore) Delete(string) error { return errors.New("the credential store is locked") }

// A selection whose configuration write fails, and whose Token then cannot be
// removed, keeps that failed undo recognizable beside its failure, and so
// does a credential reader that a failed transaction could not remove, so
// that a report can state each one.
func TestFailedUndosStayRecognizable(t *testing.T) {
	before := setupConfiguration()
	delete(before.Clients, claude.ID)
	failure := errors.New("configuration write failed")
	store := &configStoreStub{commitErr: failure}
	store.bindConfiguration(t, before)
	syncer := Synchronizer{Registry: builtinClients, Config: store, Secrets: undoRefusingStore{secret.NewMemoryStore()}, Discovery: setupDiscovery(nil)}
	_, _, _, err := syncer.SelectModel(t.Context(), before, claude.ID, "claude", "", domain.ConnectionChange{}, "new-token")
	undo, recognized := errors.AsType[*platform.RestoreError](err)
	if !errors.Is(err, failure) || !recognized || undo.Undo != "removing the Token that this command stored for provider team" {
		t.Fatalf("selection error = %v", err)
	}
	if err := undoCreatedEntrypoint(func() error { return os.ErrPermission }); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("a failed reader removal = %v", err)
	} else if undo, recognized := errors.AsType[*platform.RestoreError](err); !recognized || undo.Undo != "removing the credential reader that this command installed" {
		t.Fatalf("a failed reader removal = %v", err)
	}
	if err := undoCreatedEntrypoint(func() error { return nil }); err != nil {
		t.Fatalf("a reader removal that succeeded = %v", err)
	}
}
