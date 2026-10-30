package synchronization

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secret"

	"aigw-cli/internal/store"
	surfaceidentity "aigw-cli/internal/surface"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type staticDiscovery struct {
	result     discovery.Result
	onDiscover func()
}

func (d staticDiscovery) Discover() discovery.Result {
	if d.onDiscover != nil {
		d.onDiscover()
	}
	return d.result
}

type recordingRunner struct {
	plans []process.Plan
	err   error
}

func (r *recordingRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	r.plans = append(r.plans, plan)
	return nil, r.err
}

type runnerFunc func(context.Context, process.Plan) error

func (r *recordingRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := r.RunCapture(ctx, plan)
	return output, nil, err
}

func (run runnerFunc) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	return nil, nil, run(ctx, plan)
}

func (run runnerFunc) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	return nil, run(ctx, plan)
}

type configStoreStub struct {
	snapshot   store.Snapshot
	captureErr error
	commitErr  error
	restoreErr error
	onCommit   func()
	commits    int
	restores   int
}

func (s *configStoreStub) CaptureSnapshot() (store.Snapshot, error) {
	return s.snapshot, s.captureErr
}

func (s *configStoreStub) bindConfiguration(t *testing.T, cfg domain.Config) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	snapshot, err := configStore.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.snapshot = snapshot
}

func (s *configStoreStub) Commit(store.Snapshot, domain.Config) (store.Snapshot, error) {
	s.commits++
	if s.onCommit != nil {
		s.onCommit()
	}
	return store.Snapshot{}, s.commitErr
}

func (s *configStoreStub) RestoreSnapshot(store.Snapshot, store.Snapshot) error {
	s.restores++
	return s.restoreErr
}

func testConfig(target string) domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["gpt"] = domain.Route{Label: "GPT", Account: "gateway", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	return cfg
}

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: runtime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

func targetDiscovery(target string) staticDiscovery {
	return staticDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
		ID: string(codex.HomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target, Present: true, AutoManaged: true,
	}}}}
}

func TestWithdrawDefaultsToEveryAdmittedClientAndRejectsUnknownClients(t *testing.T) {
	syncer := Synchronizer{Registry: builtinClients}
	cfg := domain.NewConfig(clients.Admission())
	for _, clientID := range syncer.ClientIDs() {
		cfg.SetClientActivation(clientID, true, "", nil)
	}
	if err := syncer.Withdraw(&cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Clients) != 0 {
		t.Fatalf("adapters after full withdrawal = %#v", cfg.Clients)
	}
	if err := syncer.Withdraw(&cfg, "unknown"); err == nil || !strings.Contains(err.Error(), "no admitted operational adapter") {
		t.Fatalf("unknown withdrawal error = %v", err)
	}
}

func TestCommitRestoresTargetRemovedFromAdapter(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "codex", "configuration.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("model_provider = \"native\"\nmodel = \"gpt-native\"\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	configStore := store.NewStore(filepath.Join(dir, "aigw.toml"), clients.Admission())
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(target)}
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	runtime, err := before.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	delete(after.Clients, codex.ID)
	if err := syncer.Commit(context.Background(), before, after, "test"); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("restored target = %q, %v; want %q", restored, err, original)
	}
	if _, err := os.Stat(target + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar remains after adapter removal: %v", err)
	}
	stored, err := configStore.Load()
	if err != nil || stored.Clients[codex.ID].Enabled {
		t.Fatalf("stored config still enables Codex: %#v, %v", stored.Clients, err)
	}
}

func TestCancelledCommitPreservesConfiguration(t *testing.T) {
	for _, operation := range []string{"commit", "projection", "repair"} {
		t.Run(operation, func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
			before := testConfig("")
			before.Clients = map[string]domain.ClientBinding{}
			if err := configStore.Save(before); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(configStore.Path())
			if err != nil {
				t.Fatal(err)
			}
			after := before.Clone()
			after.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
			syncer := Synchronizer{Registry: builtinClients, Config: configStore}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			switch operation {
			case "commit":
				err = syncer.Commit(ctx, before, after, "test")
			case "projection":
				err = syncer.CommitProjection(ctx, before, after, "test")
			case "repair":
				err = syncer.CommitProjection(ctx, before, after, "repair")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s error = %v, want cancellation", operation, err)
			}
			current, readErr := os.ReadFile(configStore.Path())
			if readErr != nil || !bytes.Equal(current, original) {
				t.Errorf("cancelled %s changed configuration: %v", operation, readErr)
			}
		})
	}
}

func TestCommitPersistenceFailureBoundaries(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	// The committed configuration must itself be savable, so the injected
	// persistence failures are the ones that stop the commit.
	after := domain.NewConfig(clients.Admission())
	after.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	after.Routes["gpt"] = domain.Route{Label: "GPT", Account: "gateway", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	want := errors.New("failure")
	tests := []struct {
		name  string
		store *configStoreStub
	}{
		{name: "capture", store: &configStoreStub{captureErr: want}},
		{name: "commit", store: &configStoreStub{commitErr: want}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := (Synchronizer{Registry: builtinClients, Config: test.store}).Commit(context.Background(), before, after, "test"); !errors.Is(err, want) {
				t.Fatalf("Commit() error = %v", err)
			}
		})
	}
}

type cancellingConfigStore struct {
	store.Store
	phase  string
	cancel context.CancelFunc
}

func (configStore cancellingConfigStore) CaptureSnapshot() (store.Snapshot, error) {
	snapshot, err := configStore.Store.CaptureSnapshot()
	if configStore.phase == "snapshot" {
		configStore.cancel()
	}
	return snapshot, err
}

func (configStore cancellingConfigStore) Commit(before store.Snapshot, cfg domain.Config) (store.Snapshot, error) {
	snapshot, err := configStore.Store.Commit(before, cfg)
	if configStore.phase == "persistence" {
		configStore.cancel()
	}
	return snapshot, err
}

func TestCommitCancellationAtPersistenceAndProjectionAdmission(t *testing.T) {
	for _, phase := range []string{"snapshot", "preflight", "persistence"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			root := t.TempDir()
			target := filepath.Join(root, "config.toml")
			before := testConfig(target)
			before.Clients = map[string]domain.ClientBinding{}
			after := testConfig(target)
			configStore := cancellingConfigStore{Store: store.NewStore(filepath.Join(root, "aigw.toml"), clients.Admission()), phase: phase, cancel: cancel}
			if err := configStore.Save(before); err != nil {
				t.Fatal(err)
			}
			for suffix, data := range map[string]string{".bak": "prior backup", ".verified.json": "{}\n"} {
				if err := os.WriteFile(configStore.Path()+suffix, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			original, err := configStore.Store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(root, "aigw")}
			if phase == "preflight" {
				discovered := targetDiscovery(target)
				discovered.onDiscover = cancel
				syncer.Discovery = discovered
			}
			if err := syncer.CommitProjection(ctx, before, after, "sync"); !errors.Is(err, context.Canceled) {
				t.Errorf("commit error = %v, want cancellation", err)
			}
			observed, err := configStore.Store.CaptureSnapshot()
			if err != nil || !observed.Config.Equal(original.Config) || !observed.Backup.Equal(original.Backup) || !observed.Verified.Equal(original.Verified) {
				t.Errorf("cancelled transaction did not preserve configuration, backup and checkpoint: %v", err)
			}
			if files, err := os.ReadDir(root); err != nil || len(files) != 3 {
				t.Errorf("cancelled projection left unexpected files: %v, %v", files, err)
			}
		})
	}
}

func TestCommitReportsSynchronizationRollbackFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := domain.NewConfig(clients.Admission())
	after := testConfig(target)
	want := errors.New("restore failed")
	store := &configStoreStub{restoreErr: want, onCommit: func() {
		if err := os.WriteFile(target, []byte("invalid TOML {"), 0o600); err != nil {
			t.Fatal(err)
		}
	}}
	err := (Synchronizer{Registry: builtinClients, Config: store, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}).Commit(context.Background(), before, after, "change")
	if err == nil || !strings.Contains(err.Error(), "synchronization failed") || !strings.Contains(err.Error(), "rollback also failed") {
		t.Fatalf("Commit() error = %v", err)
	}
	if store.commits != 1 || store.restores != 1 || !errors.Is(err, want) {
		t.Fatalf("post-preflight failure: commits=%d restores=%d error=%v", store.commits, store.restores, err)
	}
	var recovery interface{ ConfigurationRestored() bool }
	if !errors.As(err, &recovery) || recovery.ConfigurationRestored() {
		t.Fatalf("incomplete rollback was not typed accurately: %v", err)
	}
}

func TestCommitReportsConfigurationRestoredAfterProjectionFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &configStoreStub{onCommit: func() {
		if err := os.WriteFile(target, []byte("invalid TOML {"), 0o600); err != nil {
			t.Fatal(err)
		}
	}}
	err := (Synchronizer{Registry: builtinClients,
		Config: store, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
	}).Commit(t.Context(), domain.NewConfig(clients.Admission()), testConfig(target), "change")
	var recovery interface{ ConfigurationRestored() bool }
	if !errors.As(err, &recovery) || !recovery.ConfigurationRestored() || store.commits != 1 || store.restores != 1 {
		t.Fatalf("restored configuration was not typed accurately: commits=%d restores=%d error=%v", store.commits, store.restores, err)
	}
}

func TestCommitRejectsProjectionConflictBeforePersistence(t *testing.T) {
	target := t.TempDir()
	before := domain.NewConfig(clients.Admission())
	after := testConfig(target)
	store := &configStoreStub{}
	err := (Synchronizer{Registry: builtinClients, Config: store, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}).Commit(t.Context(), before, after, "route")
	if err == nil || store.commits != 0 || store.restores != 0 || strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("preflight must reject without writes or compensation: commits=%d restores=%d error=%v", store.commits, store.restores, err)
	}
}

func TestCommitProjectionPreservesConfigurationWhenPreflightFails(t *testing.T) {
	target := t.TempDir()
	before := testConfig(target)
	after := before.Clone()
	account := after.Accounts["gateway"]
	account.Label = "Renamed gateway"
	after.Accounts["gateway"] = account
	configPath := filepath.Join(t.TempDir(), "aigw.toml")
	configStore := store.NewStore(configPath, clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(target)}
	if err := syncer.CommitProjection(context.Background(), before, after, "sync"); err == nil || !strings.Contains(err.Error(), "preflight") || strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("preflight failure must not claim rollback: %v", err)
	}
	restored, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("failed repair retained a partial configuration commit:\n%s", restored)
	}
}

type secretReadStub struct{ err error }

func (s secretReadStub) Get(string) (string, error)  { return "", s.err }
func (secretReadStub) Set(string, string) error      { return nil }
func (secretReadStub) Delete(string) error           { return nil }
func (s secretReadStub) Exists(string) (bool, error) { return s.err == nil, s.err }

func TestCommitProjectsAndRestoresClaudeOfficialSettings(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(settingsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before := domain.NewConfig(clients.Admission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	before.Routes["claude"] = domain.Route{
		Label: "Claude", Account: "gateway", Model: "claude-team",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	before.SetSelectedRoute(claude.ID, "claude", "")
	after := before.Clone()
	after.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	configStore := store.NewStore(filepath.Join(dir, "aigw.toml"), clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	aigwExecutable := filepath.Join(t.TempDir(), "secret-token-aigw")
	credentials := secret.NewMemoryStore()
	const accountToken = "synthetic-account-token-not-for-authentication"
	if err := credentials.Set("gateway", accountToken); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Secrets: credentials, Host: hostAt(dir), AIGWExecutable: aigwExecutable}
	if err := syncer.Commit(t.Context(), before, after, "enable Claude"); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Theme        string            `json:"theme"`
		Model        string            `json:"model"`
		APIKeyHelper string            `json:"apiKeyHelper"`
		Environment  map[string]string `json:"env"`
	}
	decoder := json.NewDecoder(bytes.NewReader(projected))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	projection, err := after.ResolveRuntime(claude.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := secret.Command(aigwExecutable, claude.ID, projection.CredentialProjectionFingerprint(claude.ID), runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	if document.Theme != "dark" || document.Model != "claude-team" || document.APIKeyHelper != helper ||
		len(document.Environment) != 1 || document.Environment["ANTHROPIC_BASE_URL"] != "https://gateway.test" {
		t.Fatal("projected settings differ from the selected route, native credential command, or preserved user settings")
	}
	if bytes.Contains(projected, []byte(accountToken)) {
		t.Fatal("projected settings contain the synthetic Account Token")
	}
	if err := syncer.Commit(t.Context(), after, before, "disable Claude"); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(settingsPath)
	if err != nil || !strings.Contains(string(restored), `"theme": "dark"`) {
		t.Fatalf("restored settings = %s, %v", restored, err)
	}
	if _, err := os.Stat(settingsPath + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("Claude settings state remains: %v", err)
	}
}

func TestCommitPreservesConfigurationWhenClaudePreflightFails(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"apiKeyHelper":"foreign"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before := domain.NewConfig(clients.Admission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	before.Routes["claude"] = domain.Route{
		Label: "Claude", Account: "gateway", Model: "claude-team",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	before.SetSelectedRoute(claude.ID, "claude", "")
	after := before.Clone()
	after.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	configStore := store.NewStore(filepath.Join(dir, "aigw.toml"), clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Host: hostAt(dir), AIGWExecutable: "/usr/local/bin/aigw"}
	err := syncer.Commit(context.Background(), before, after, "enable Claude")
	if err == nil || !strings.Contains(err.Error(), "preflight") || strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Commit() error = %v", err)
	}
	stored, loadErr := configStore.Load()
	if loadErr != nil || stored.Clients[claude.ID].Enabled {
		t.Fatalf("configuration was not rolled back: %#v, %v", stored, loadErr)
	}
}
