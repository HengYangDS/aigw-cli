package synchronization

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
)

func TestFinalizeCredentialEntrypointRejectsMissingActiveHelper(t *testing.T) {
	root := t.TempDir()
	syncer := Synchronizer{CredentialPath: filepath.Join(root, "data", "credential", "aigw")}
	err := syncer.finalizeCredentialEntrypoint(testConfig(filepath.Join(root, "codex.toml")))
	if err == nil || !strings.Contains(err.Error(), "disappeared after client projection") {
		t.Fatalf("missing active helper finalization = %v", err)
	}
}

type removedEntrypointAdapter struct {
	client.Adapter
	target      string
	reader      string
	rollbackErr error
}

func (adapter removedEntrypointAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: configuration.ClientCodex, Label: "Codex", EndpointProtocols: []configuration.EndpointProtocol{configuration.ProtocolOpenAIResponses}}
}

func (adapter removedEntrypointAdapter) Plan(client.Dependencies, configuration.Config, configuration.Config) ([]client.ProjectionPlan, error) {
	return []client.ProjectionPlan{{Client: configuration.ClientCodex, Target: adapter.target, Action: "update"}}, nil
}

func (adapter removedEntrypointAdapter) Apply(context.Context, client.Dependencies, configuration.Config, configuration.Config) (client.ProjectionReceipt, error) {
	previous, err := os.ReadFile(adapter.target)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(adapter.target, []byte("reader="+adapter.reader), 0o600); err != nil {
		return nil, err
	}
	if err := credential.RemoveEntrypoint(adapter.reader); err != nil {
		return nil, err
	}
	return projectionUndo(func() error {
		if adapter.rollbackErr != nil {
			return adapter.rollbackErr
		}
		return os.WriteFile(adapter.target, previous, 0o600)
	}), nil
}

type projectionUndo func() error

func (undo projectionUndo) Rollback() error { return undo() }

func TestProjectionCompensatesMissingEntrypointAfterApply(t *testing.T) {
	for _, mode := range []string{"commit", "reconcile"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "aigw")
			target := filepath.Join(root, "codex.toml")
			previous := []byte("model_provider = \"native\"\n")
			if err := os.WriteFile(source, []byte("executable fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, previous, 0o600); err != nil {
				t.Fatal(err)
			}
			reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
			if err != nil {
				t.Fatal(err)
			}
			before := testConfig(target)
			before.SetClientActivation(configuration.ClientCodex, false, "", nil)
			store := configuration.NewStore(filepath.Join(root, "config.toml"))
			if err := store.Save(before); err != nil {
				t.Fatal(err)
			}
			adapter := removedEntrypointAdapter{target: target, reader: reader}
			registry, err := client.NewRegistry([]configuration.ClientSpec{adapter.Spec()}, adapter)
			if err != nil {
				t.Fatal(err)
			}
			syncer := Synchronizer{Config: store, Registry: registry, AIGWExecutable: source, CredentialPath: reader}
			after := testConfig(target)
			if mode == "commit" {
				err = syncer.CommitProjection(t.Context(), before, after, "sync")
			} else {
				err = syncer.ReconcileClient(t.Context(), after, configuration.ClientCodex)
			}
			if err == nil {
				t.Fatal("missing reader after projection was accepted")
			}
			if mode == "commit" {
				stored, loadErr := store.Load()
				if loadErr != nil || stored.Clients[configuration.ClientCodex].Enabled {
					t.Fatalf("failed synchronization retained enabled Codex: %v, %v", stored.Clients[configuration.ClientCodex], loadErr)
				}
				var recovery interface{ ConfigurationRestored() bool }
				if !errors.As(err, &recovery) || !recovery.ConfigurationRestored() {
					t.Fatalf("finalization lost successful configuration restoration: %v", err)
				}
			}
			if got, readErr := os.ReadFile(target); readErr != nil || !bytes.Equal(got, previous) {
				t.Fatalf("failed synchronization retained client projection %q: %v", got, readErr)
			}
		})
	}
}

func TestCredentialFinalizationReportsConfigurationOutcomeDespiteCompensationFailure(t *testing.T) {
	const privatePath = "/private/finalization/configuration.toml"
	cause := &os.PathError{Op: "write", Path: privatePath, Err: os.ErrPermission}
	for _, tc := range []struct {
		name          string
		configErr     error
		projectionErr error
		want          string
		restored      bool
	}{
		{"configuration", cause, nil, "Configuration restoration was incomplete", false},
		{"projection", nil, cause, "Configuration was restored", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			adapter := removedEntrypointAdapter{target: target, reader: reader, rollbackErr: tc.projectionErr}
			registry, err := client.NewRegistry([]configuration.ClientSpec{adapter.Spec()}, adapter)
			if err != nil {
				t.Fatal(err)
			}
			store := &configStoreStub{restoreErr: tc.configErr}
			syncer := Synchronizer{Config: store, Registry: registry, AIGWExecutable: source, CredentialPath: reader}
			err = syncer.CommitProjection(t.Context(), configuration.NewConfig(), testConfig(target), "sync")
			var recovery interface{ ConfigurationRestored() bool }
			if !errors.Is(err, cause) || !errors.As(err, &recovery) || recovery.ConfigurationRestored() != tc.restored || store.restores != 1 {
				t.Fatalf("finalization outcome: restores=%d error=%v", store.restores, err)
			}
			for _, jsonMode := range []bool{false, true} {
				var out bytes.Buffer
				renderer := presentation.New(&out, false)
				presentation.RenderError(renderer, fmt.Errorf("sync: %w", err), jsonMode)
				if renderer.Err() != nil || !strings.Contains(out.String(), tc.want) || !strings.Contains(out.String(), "aigw doctor") || strings.Contains(out.String(), privatePath) {
					t.Fatalf("json=%t finalization diagnostic=%q error=%v", jsonMode, out.String(), renderer.Err())
				}
			}
		})
	}
}

func TestFinalizeCredentialEntrypointRejectsInvalidBinding(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(filepath.Join(root, "codex.toml"))
	delete(cfg.Routes, "gpt")
	syncer := Synchronizer{CredentialPath: filepath.Join(root, "data", "credential", "aigw")}
	if err := syncer.finalizeCredentialEntrypoint(cfg); err == nil || !strings.Contains(err.Error(), "inspect credential entrypoint") {
		t.Fatalf("invalid binding finalization = %v", err)
	}
}
