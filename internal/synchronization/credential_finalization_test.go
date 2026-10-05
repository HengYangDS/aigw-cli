package synchronization

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

func TestExplicitTokenVerificationRefusesUnresolvedReaderFromNativeBinding(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "codex.toml"))
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Authentication = configuration.AuthenticationClientNative
	binding.ModelProvider = "amazon-bedrock"
	cfg.Clients[configuration.ClientCodex] = binding
	cfg.Accounts["explicit"] = cfg.Accounts["gateway"]
	cfg.Routes["explicit"] = cfg.Routes["gpt"]
	route := cfg.Routes["explicit"]
	route.Account = "explicit"
	cfg.Routes["explicit"] = route
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "explicit")
	if err != nil || !runtime.UsesAIGWCredentialStore() {
		t.Fatalf("explicit account-token runtime = %+v, %v", runtime, err)
	}
	problem := errors.New("identity source unavailable")
	syncer := Synchronizer{ResolveCredentialPath: func() (string, error) { return "", problem }}
	if _, err := syncer.Verify(t.Context(), cfg, configuration.ClientCodex, runtime, "explicit"); !errors.Is(err, problem) {
		t.Fatalf("explicit Token verification used the public fallback: %v", err)
	}
	if current := cfg.Clients[configuration.ClientCodex]; current.Authentication != binding.Authentication || current.Route != binding.Route || current.ModelProvider != binding.ModelProvider {
		t.Fatal("explicit verification changed the configured native binding")
	}
}

func TestUnresolvedReaderIdentityRefusesConsumersBeforeMutation(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprint(empty), func(t *testing.T) {
			root := t.TempDir()
			cfg := testConfig(filepath.Join(root, "codex.toml"))
			store := &configStoreStub{}
			problem := errors.New("identity source unavailable")
			syncer := Synchronizer{Config: store, ResolveCredentialPath: func() (string, error) {
				if empty {
					return "", nil
				}
				return "", problem
			}}
			runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, operation := range []func() error{
				func() error { _, err := syncer.Plan(cfg, cfg); return err },
				func() error { _, err := syncer.CredentialEntrypointPlan(cfg); return err },
				func() error { return syncer.ReconcileClient(t.Context(), cfg, configuration.ClientCodex) },
				func() error { return syncer.CommitProjection(t.Context(), cfg, cfg, "test") },
				func() error {
					_, err := syncer.Verify(t.Context(), cfg, configuration.ClientCodex, runtime, "")
					return err
				},
			} {
				if err := operation(); err == nil || !empty && !errors.Is(err, problem) {
					t.Fatalf("unresolved reader admitted: %v", err)
				}
			}
			if status := syncer.Inspect(t.Context(), cfg, configuration.ClientCodex, runtime); status.Ready || !strings.Contains(status.Issue, "reader identity") {
				t.Fatalf("unresolved reader status = %+v", status)
			}
			if store.commits != 0 {
				t.Fatal("reader resolution failure changed configuration")
			}
			for _, unused := range []configuration.Config{configuration.NewConfig(), cfg.Clone()} {
				if len(unused.Clients) > 0 {
					binding := unused.Clients[configuration.ClientCodex]
					binding.CredentialCommand = filepath.Join(root, "external-reader")
					unused.Clients[configuration.ClientCodex] = binding
				}
				if action, err := syncer.CredentialEntrypointPlan(unused); err != nil || action != CredentialEntrypointUnchanged {
					t.Fatalf("unused reader plan = %q, %v", action, err)
				}
			}
		})
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
	return []client.ProjectionPlan{{Client: configuration.ClientCodex, Target: adapter.target, Action: "update", ChangesState: true}}, nil
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

type convergedProjectionAdapter struct {
	removedEntrypointAdapter
	applied *int
}

func (adapter convergedProjectionAdapter) Plan(client.Dependencies, configuration.Config, configuration.Config) ([]client.ProjectionPlan, error) {
	return []client.ProjectionPlan{{Client: configuration.ClientCodex, Target: adapter.target, Action: "unchanged"}}, nil
}

func (adapter convergedProjectionAdapter) Apply(context.Context, client.Dependencies, configuration.Config, configuration.Config) (client.ProjectionReceipt, error) {
	*adapter.applied++
	return projectionUndo(func() error { return nil }), nil
}

func TestConvergedProjectionValidatesReaderWithoutApplying(t *testing.T) {
	for _, tc := range []struct {
		name, changedTarget string
		duringCommit        bool
		commits, restores   int
	}{
		{"unchanged", "", false, 1, 0},
		{"source", "source", false, 0, 0},
		{"reader", "reader", false, 0, 0},
		{"reader-during-commit", "", true, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "aigw")
			if err := os.WriteFile(source, []byte("executable fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := credential.EnsureEntrypoint(source, reader); err != nil {
				t.Fatal(err)
			}
			if path := map[string]string{"source": source, "reader": reader}[tc.changedTarget]; path != "" {
				if err := os.WriteFile(path, []byte("changed executable"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			applied := 0
			adapter := convergedProjectionAdapter{target: filepath.Join(root, "codex.toml"), applied: &applied}
			registry, err := client.NewRegistry([]configuration.ClientSpec{adapter.Spec()}, adapter)
			if err != nil {
				t.Fatal(err)
			}
			store := &configStoreStub{}
			if tc.duringCommit {
				store.onCommit = func() {
					if err := credential.RemoveEntrypoint(reader); err != nil {
						t.Fatal(err)
					}
				}
			}
			syncer := Synchronizer{Config: store, Registry: registry, AIGWExecutable: source, CredentialPath: reader}
			cfg := testConfig(adapter.target)
			err = syncer.CommitProjection(t.Context(), cfg, cfg, "sync")
			if wantError := tc.changedTarget != "" || tc.duringCommit; (err != nil) != wantError || store.commits != tc.commits || store.restores != tc.restores {
				t.Fatalf("synchronization = %v, commits = %d, restores = %d; want error = %t, commits = %d, restores = %d", err, store.commits, store.restores, wantError, tc.commits, tc.restores)
			}
			if applied != 0 {
				t.Fatalf("converged projection was unnecessarily applied %d times", applied)
			}
		})
	}
}

func TestConvergedProjectionRechecksTargetsAfterConfigurationCommit(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("invalid=%t", invalid), func(t *testing.T) {
			root := t.TempDir()
			source, target := filepath.Join(root, "aigw"), filepath.Join(root, "codex.toml")
			if err := os.WriteFile(source, []byte("executable fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
			if err != nil {
				t.Fatal(err)
			}
			store := &configStoreStub{}
			syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: reader}
			cfg := testConfig(target)
			if err := syncer.CommitProjection(t.Context(), cfg, cfg, "initial sync"); err != nil {
				t.Fatal(err)
			}
			dependencies, err := syncer.clientDependencies([]string{configuration.ClientCodex}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			plans, err := syncer.registry().Plan(dependencies, cfg, cfg, configuration.ClientCodex)
			if err != nil || len(plans) == 0 || slices.ContainsFunc(plans, func(plan client.ProjectionPlan) bool { return plan.ChangesState }) {
				t.Fatalf("initial managed projection plan = %v, %v", plans, err)
			}
			store.commits = 0
			store.onCommit = func() {
				if invalid {
					err = os.WriteFile(target, []byte("invalid TOML {"), 0o600)
				} else {
					err = os.Remove(target)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			err = syncer.CommitProjection(t.Context(), cfg, cfg, "sync")
			var recovery interface{ ConfigurationRestored() bool }
			if !errors.As(err, &recovery) || !recovery.ConfigurationRestored() || store.commits != 1 || store.restores != 1 {
				t.Fatalf("target mutation synchronization = %v, commits/restores = %d/%d", err, store.commits, store.restores)
			}
			got, readErr := os.ReadFile(target)
			if invalid {
				if readErr != nil || !bytes.Equal(got, []byte("invalid TOML {")) {
					t.Fatalf("external target edit was overwritten: %q, %v", got, readErr)
				}
			} else if !errors.Is(readErr, os.ErrNotExist) {
				t.Fatalf("removed target was recreated without ownership: %q, %v", got, readErr)
			}
		})
	}
}

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
