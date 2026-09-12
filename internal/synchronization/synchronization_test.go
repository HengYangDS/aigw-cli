package synchronization

import (
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	surfaceidentity "aigw-cli/internal/surface"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func (run runnerFunc) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	return nil, run(ctx, plan)
}

type configStoreStub struct {
	captureErr error
	commitErr  error
	restoreErr error
	onCommit   func()
	commits    int
	restores   int
}

func (s *configStoreStub) CaptureSnapshot() (configuration.Snapshot, error) {
	return configuration.Snapshot{}, s.captureErr
}

func (s *configStoreStub) Commit(configuration.Snapshot, configuration.Config) (configuration.Snapshot, error) {
	s.commits++
	if s.onCommit != nil {
		s.onCommit()
	}
	return configuration.Snapshot{}, s.commitErr
}

func (s *configStoreStub) RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error {
	s.restores++
	return s.restoreErr
}

func testConfig(target string) configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

func targetDiscovery(target string) staticDiscovery {
	return staticDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
		ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target, Present: true, AutoManaged: true,
	}}}}
}

func TestWithdrawDefaultsToEveryAdmittedClientAndRejectsUnknownClients(t *testing.T) {
	syncer := Synchronizer{}
	cfg := configuration.NewConfig()
	for _, clientID := range syncer.ClientIDs() {
		cfg.Adapters[clientID] = configuration.AdapterConfig{Enabled: true}
	}
	if err := syncer.Withdraw(&cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Adapters) != 0 {
		t.Fatalf("adapters after full withdrawal = %#v", cfg.Adapters)
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
	store := configuration.NewStore(filepath.Join(dir, "aigw.toml"))
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target)}
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	runtime, err := before.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	delete(after.Adapters, configuration.ClientCodex)
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
	stored, err := store.Load()
	if err != nil || stored.Adapters[configuration.ClientCodex].Enabled {
		t.Fatalf("stored config still enables Codex: %#v, %v", stored.Adapters, err)
	}
}

func TestCancelledCommitPreservesConfiguration(t *testing.T) {
	for _, operation := range []string{"commit", "projection", "repair"} {
		t.Run(operation, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
			before := testConfig("")
			before.Adapters = map[string]configuration.AdapterConfig{}
			if err := store.Save(before); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			after := before.Clone()
			after.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
			syncer := Synchronizer{Config: store}
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
			current, readErr := os.ReadFile(store.Path())
			if readErr != nil || !bytes.Equal(current, original) {
				t.Errorf("cancelled %s changed configuration: %v", operation, readErr)
			}
		})
	}
}

func TestCommitPersistenceFailureBoundaries(t *testing.T) {
	before := configuration.NewConfig()
	after := before.Clone()
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
			if err := (Synchronizer{Config: test.store}).Commit(context.Background(), before, after, "test"); !errors.Is(err, want) {
				t.Fatalf("Commit() error = %v", err)
			}
		})
	}
}

type cancellingConfigStore struct {
	configuration.Store
	phase  string
	cancel context.CancelFunc
}

func (store cancellingConfigStore) CaptureSnapshot() (configuration.Snapshot, error) {
	snapshot, err := store.Store.CaptureSnapshot()
	if store.phase == "snapshot" {
		store.cancel()
	}
	return snapshot, err
}

func (store cancellingConfigStore) Commit(before configuration.Snapshot, cfg configuration.Config) (configuration.Snapshot, error) {
	snapshot, err := store.Store.Commit(before, cfg)
	if store.phase == "persistence" {
		store.cancel()
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
			before.Adapters = map[string]configuration.AdapterConfig{}
			after := testConfig(target)
			store := cancellingConfigStore{Store: configuration.NewStore(filepath.Join(root, "aigw.toml")), phase: phase, cancel: cancel}
			if err := store.Save(before); err != nil {
				t.Fatal(err)
			}
			for suffix, data := range map[string]string{".bak": "prior backup", ".verified.json": "{}\n"} {
				if err := os.WriteFile(store.Path()+suffix, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			original, err := store.Store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(root, "aigw")}
			if phase == "preflight" {
				discovered := targetDiscovery(target)
				discovered.onDiscover = cancel
				syncer.Discovery = discovered
			}
			if err := syncer.CommitProjection(ctx, before, after, "sync"); !errors.Is(err, context.Canceled) {
				t.Errorf("commit error = %v, want cancellation", err)
			}
			observed, err := store.Store.CaptureSnapshot()
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
	before := configuration.NewConfig()
	after := testConfig(target)
	want := errors.New("restore failed")
	store := &configStoreStub{restoreErr: want, onCommit: func() {
		if err := os.WriteFile(target, []byte("invalid TOML {"), 0o600); err != nil {
			t.Fatal(err)
		}
	}}
	err := (Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}).Commit(context.Background(), before, after, "change")
	if err == nil || !strings.Contains(err.Error(), "synchronization failed") || !strings.Contains(err.Error(), "rollback also failed") {
		t.Fatalf("Commit() error = %v", err)
	}
	if store.commits != 1 || store.restores != 1 || !errors.Is(err, want) {
		t.Fatalf("post-preflight failure: commits=%d restores=%d error=%v", store.commits, store.restores, err)
	}
}

func TestCommitRejectsProjectionConflictBeforePersistence(t *testing.T) {
	target := t.TempDir()
	before := configuration.NewConfig()
	after := testConfig(target)
	store := &configStoreStub{}
	err := (Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}).Commit(t.Context(), before, after, "route")
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
	store := configuration.NewStore(configPath)
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target)}
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
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-team"}
	before.Routes[configuration.ClientClaude] = "claude"
	after := before.Clone()
	after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	store := configuration.NewStore(filepath.Join(dir, "aigw.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	aigwExecutable := filepath.Join(t.TempDir(), "aigw")
	syncer := Synchronizer{Config: store, ClaudeSettingsPath: settingsPath, AIGWExecutable: aigwExecutable}
	if err := syncer.Commit(context.Background(), before, after, "enable Claude"); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(settingsPath)
	if err != nil || !strings.Contains(string(projected), `"ANTHROPIC_BASE_URL": "https://gateway.test"`) {
		t.Fatalf("projected settings = %s, %v", projected, err)
	}
	var document map[string]any
	if err := json.Unmarshal(projected, &document); err != nil {
		t.Fatal(err)
	}
	helper, ok := document["apiKeyHelper"].(string)
	projection, err := after.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !strings.Contains(helper, aigwExecutable) || !strings.HasSuffix(helper, " credential claude "+projection.CredentialProjectionFingerprint(configuration.ClientClaude)) {
		t.Fatalf("apiKeyHelper = %#v", document["apiKeyHelper"])
	}
	if strings.Contains(string(projected), "token") || strings.Contains(string(projected), "secret") {
		t.Fatalf("projected settings leaked credential material: %s", projected)
	}
	if err := syncer.Commit(context.Background(), after, before, "disable Claude"); err != nil {
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
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-team"}
	before.Routes[configuration.ClientClaude] = "claude"
	after := before.Clone()
	after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	store := configuration.NewStore(filepath.Join(dir, "aigw.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, ClaudeSettingsPath: settingsPath, AIGWExecutable: "/usr/local/bin/aigw"}
	err := syncer.Commit(context.Background(), before, after, "enable Claude")
	if err == nil || !strings.Contains(err.Error(), "preflight") || strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Commit() error = %v", err)
	}
	stored, loadErr := store.Load()
	if loadErr != nil || stored.Adapters[configuration.ClientClaude].Enabled {
		t.Fatalf("configuration was not rolled back: %#v, %v", stored, loadErr)
	}
}
