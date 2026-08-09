package synchronization

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

type staticDiscovery struct{ result discovery.Result }

func (d staticDiscovery) Discover() discovery.Result { return d.result }

type failingRunner struct{ err error }

func (r failingRunner) Run(_ context.Context, _ process.Plan) error { return r.err }

type recordingRunner struct {
	plans []process.Plan
	err   error
}

func (r *recordingRunner) Run(_ context.Context, plan process.Plan) error {
	r.plans = append(r.plans, plan)
	return r.err
}

type configStoreStub struct {
	captures   []configuration.Snapshot
	captureAt  int
	captureErr map[int]error
	saveErr    error
	restoreErr error
	saved      []configuration.Config
	restored   [][2]configuration.Snapshot
}

func (s *configStoreStub) CaptureSnapshot() (configuration.Snapshot, error) {
	index := s.captureAt
	s.captureAt++
	if err := s.captureErr[index]; err != nil {
		return configuration.Snapshot{}, err
	}
	if index < len(s.captures) {
		return s.captures[index], nil
	}
	return configuration.Snapshot{}, nil
}

func (s *configStoreStub) Save(cfg configuration.Config) error {
	s.saved = append(s.saved, cfg)
	return s.saveErr
}

func (s *configStoreStub) RestoreSnapshot(before, after configuration.Snapshot) error {
	s.restored = append(s.restored, [2]configuration.Snapshot{before, after})
	return s.restoreErr
}

func testConfig(target string) configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "gateway", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

func targetDiscovery(target string) staticDiscovery {
	return staticDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
		ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target, Present: true, AutoManaged: true,
	}}}}
}

func TestProjectionChangedForPersistentCodexSemantics(t *testing.T) {
	target := filepath.Join(t.TempDir(), "configuration.toml")
	before := testConfig(target)

	disabled := before.Clone()
	delete(disabled.Adapters, configuration.ClientCodex)
	if !ProjectionChanged(before, disabled) {
		t.Fatal("adapter removal must change the projection")
	}

	purpose := before.Clone()
	profile := purpose.Profiles["gpt"]
	profile.Purpose = "display only"
	purpose.Profiles["gpt"] = profile
	if ProjectionChanged(before, purpose) {
		t.Fatal("display-only purpose must not change the projection")
	}
}

func TestRouteAndAuthenticationSemantics(t *testing.T) {
	disabled := configuration.NewConfig()
	if accountID, ok := RouteAccount(disabled); ok || accountID != "" {
		t.Fatalf("disabled route = %q, %v", accountID, ok)
	}
	invalid := configuration.NewConfig()
	invalid.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if accountID, ok := RouteAccount(invalid); ok || accountID != "" {
		t.Fatalf("invalid route = %q, %v", accountID, ok)
	}
	configured := testConfig("/target")
	if accountID, ok := RouteAccount(configured); !ok || accountID != "gateway" {
		t.Fatalf("configured route = %q, %v", accountID, ok)
	}
	if !RouteUsesAccount(configured, "gateway") || RouteUsesAccount(configured, "other") {
		t.Fatal("route account membership is incorrect")
	}
	if AuthenticationChanged(configured, disabled) {
		t.Fatal("disabling Codex must not attempt authentication")
	}
	if !AuthenticationChanged(disabled, configured) {
		t.Fatal("enabling Codex must bind authentication")
	}
	if AuthenticationChanged(configured, configured.Clone()) {
		t.Fatal("unchanged route must not bind authentication")
	}
	movedTarget := configured.Clone()
	movedTarget.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{"/other"}}
	if !AuthenticationChanged(configured, movedTarget) {
		t.Fatal("changed targets must bind authentication")
	}
	changedAccount := configured.Clone()
	changedAccount.Accounts["next"] = configuration.Account{Label: "Next", Endpoints: configuration.Endpoints{OpenAIResponses: "https://next.test/v1"}}
	profile := changedAccount.Profiles["gpt"]
	profile.Account = "next"
	changedAccount.Profiles["gpt"] = profile
	if !AuthenticationChanged(configured, changedAccount) {
		t.Fatal("changed route account must bind authentication")
	}
}

func TestProjectionPlanningAndReconciliationNoOp(t *testing.T) {
	syncer := Synchronizer{}
	cfg := configuration.NewConfig()
	plans, err := syncer.Plan(cfg, cfg)
	if err != nil || len(plans) != 0 {
		t.Fatalf("disabled plan = %#v, %v", plans, err)
	}
	if err := syncer.Reconcile(context.Background(), cfg, cfg); err != nil {
		t.Fatalf("disabled reconciliation = %v", err)
	}

	invalid := testConfig("/target")
	delete(invalid.Profiles, "gpt")
	if _, err := (Synchronizer{Discovery: staticDiscovery{}}).Plan(invalid, invalid); err == nil {
		t.Fatal("planning accepted an invalid runtime")
	}
	if err := (Synchronizer{Discovery: staticDiscovery{}}).Reconcile(context.Background(), invalid, invalid); err == nil {
		t.Fatal("reconciliation accepted an invalid runtime")
	}
}

func TestBindAuthenticationSuccessAndFailures(t *testing.T) {
	cfg := testConfig("/tmp/codex/configuration.toml")
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	syncer := Synchronizer{Secrets: secretStore, Runner: runner}
	if err := syncer.BindAuthentication(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != "/opt/codex" || runner.plans[0].Stdin != "token\n" {
		t.Fatalf("authentication plans = %#v", runner.plans)
	}
	if err := syncer.BindAuthentication(context.Background(), configuration.NewConfig()); err != nil {
		t.Fatalf("disabled authentication = %v", err)
	}

	badRuntime := cfg.Clone()
	badRuntime.Routes.Default = "missing"
	if err := syncer.BindAuthenticationTargets(context.Background(), badRuntime, nil); err == nil {
		t.Fatal("authentication accepted an invalid runtime")
	}
	if err := (Synchronizer{Secrets: fixedSecrets{}, Runner: runner}).BindAuthenticationTargets(context.Background(), cfg, cfg.Adapters[configuration.ClientCodex].Targets); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty-token error = %v", err)
	}
	runner.err = errors.New("login failed")
	if err := syncer.BindAuthenticationTargets(context.Background(), cfg, cfg.Adapters[configuration.ClientCodex].Targets); !errors.Is(err, runner.err) {
		t.Fatalf("runner error = %v", err)
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
	runtime, _, err := before.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
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

func TestCommitRestoresProjectionWhenAuthenticationFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "codex", "configuration.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before := configuration.NewConfig()
	after := testConfig(target)
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "test-token"); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{
		Config: configuration.NewStore(filepath.Join(dir, "aigw.toml")), Secrets: secretStore,
		Runner: failingRunner{err: os.ErrPermission}, Discovery: targetDiscovery(target),
	}
	err := syncer.Commit(context.Background(), before, after, "test")
	if err == nil || !strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("Commit() error = %v", err)
	}
	restored, readErr := os.ReadFile(target)
	if readErr != nil || !bytes.Equal(restored, original) {
		t.Fatalf("target after rollback = %q, %v; want %q", restored, readErr, original)
	}
	if _, err := os.Stat(target + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar remains after authentication rollback: %v", err)
	}
}

func TestPlanningAndAuthenticationRejectIncompleteDependencies(t *testing.T) {
	base := testConfig("/target")
	if _, err := (Synchronizer{}).Plan(base, base); err == nil {
		t.Fatal("expected discovery error")
	}
	syncer := Synchronizer{Discovery: staticDiscovery{}}
	before := base.Clone()
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := syncer.Plan(before, base); err == nil {
		t.Fatal("expected before-target error")
	}
	after := base.Clone()
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := syncer.Plan(base, after); err == nil {
		t.Fatal("expected after-target error")
	}
	if err := (Synchronizer{}).BindAuthenticationTargets(context.Background(), configuration.NewConfig(), nil); err == nil || !strings.Contains(err.Error(), "enabled adapter") {
		t.Fatalf("disabled authentication error = %v", err)
	}
	missingExecutable := base.Clone()
	missingExecutable.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if err := (Synchronizer{}).BindAuthenticationTargets(context.Background(), missingExecutable, nil); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("executable error = %v", err)
	}
	if err := (Synchronizer{Runner: failingRunner{}}).BindAuthenticationTargets(context.Background(), base, nil); err == nil || !strings.Contains(err.Error(), "secret store") {
		t.Fatalf("secret-store error = %v", err)
	}
	want := errors.New("token unavailable")
	if err := (Synchronizer{Runner: failingRunner{}, Secrets: failingSecrets{err: want}}).BindAuthenticationTargets(context.Background(), base, nil); !errors.Is(err, want) {
		t.Fatalf("token error = %v", err)
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
		{name: "capture before", store: &configStoreStub{captureErr: map[int]error{0: want}}},
		{name: "save", store: &configStoreStub{captureErr: map[int]error{}, saveErr: want}},
		{name: "capture after", store: &configStoreStub{captureErr: map[int]error{1: want}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := (Synchronizer{Config: test.store}).Commit(context.Background(), before, after, "test"); !errors.Is(err, want) {
				t.Fatalf("Commit() error = %v", err)
			}
		})
	}
}

func TestCommitReportsSynchronizationRollbackFailure(t *testing.T) {
	before := configuration.NewConfig()
	after := testConfig("/missing/configuration.toml")
	want := errors.New("restore failed")
	store := &configStoreStub{captureErr: map[int]error{}, restoreErr: want}
	err := (Synchronizer{Config: store, Discovery: staticDiscovery{}}).Commit(context.Background(), before, after, "change")
	if err == nil || !strings.Contains(err.Error(), "synchronization failed") || !strings.Contains(err.Error(), "rollback also failed") {
		t.Fatalf("Commit() error = %v", err)
	}
}

func TestCommitReportsAuthenticationRollbackFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := configuration.NewConfig()
	after := testConfig(target)
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	store := &configStoreStub{captureErr: map[int]error{}, restoreErr: errors.New("restore failed")}
	syncer := Synchronizer{Config: store, Secrets: secretStore, Runner: failingRunner{err: os.ErrPermission}, Discovery: targetDiscovery(target)}
	err := syncer.Commit(context.Background(), before, after, "change")
	if err == nil || !strings.Contains(err.Error(), "authentication failed") || !strings.Contains(err.Error(), "rollback also failed") || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Commit() error = %v", err)
	}
}

func TestRollbackReconcilesAndRebinds(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	after := before.Clone()
	profile := after.Profiles["gpt"]
	profile.Models[configuration.ClientCodex] = "gpt-next"
	after.Profiles["gpt"] = profile
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	store := &configStoreStub{captureErr: map[int]error{}}
	syncer := Synchronizer{Config: store, Secrets: secretStore, Runner: runner, Discovery: targetDiscovery(target)}
	if err := syncer.rollback(context.Background(), before, after, configuration.Snapshot{}, configuration.Snapshot{}, true); err != nil {
		t.Fatal(err)
	}
	if len(store.restored) != 1 || len(runner.plans) != 1 {
		t.Fatalf("restores=%d plans=%d", len(store.restored), len(runner.plans))
	}
}

type failingSecrets struct{ err error }

func (s failingSecrets) Get(string) (string, error) { return "", s.err }
func (failingSecrets) Set(string, string) error     { return nil }
func (failingSecrets) Delete(string) error          { return nil }
func (failingSecrets) Has(string) bool              { return false }

type fixedSecrets struct{}

func (fixedSecrets) Get(string) (string, error) { return "", nil }
func (fixedSecrets) Set(string, string) error   { return nil }
func (fixedSecrets) Delete(string) error        { return nil }
func (fixedSecrets) Has(string) bool            { return true }

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
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-team"}}
	before.Routes.Default = "claude"
	after := before.Clone()
	after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	store := configuration.NewStore(filepath.Join(dir, "aigw.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, ClaudeSettingsPath: settingsPath}
	if err := syncer.Commit(context.Background(), before, after, "enable Claude"); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(settingsPath)
	if err != nil || !strings.Contains(string(projected), `"ANTHROPIC_BASE_URL": "https://gateway.test"`) || !strings.Contains(string(projected), `"apiKeyHelper": "aigw credential claude"`) {
		t.Fatalf("projected settings = %s, %v", projected, err)
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

func TestCommitRollsBackConfigurationWhenClaudeProjectionFails(t *testing.T) {
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
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude}
	before.Routes.Default = "claude"
	after := before.Clone()
	after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	store := configuration.NewStore(filepath.Join(dir, "aigw.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, ClaudeSettingsPath: settingsPath}
	err := syncer.Commit(context.Background(), before, after, "enable Claude")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Commit() error = %v", err)
	}
	stored, loadErr := store.Load()
	if loadErr != nil || stored.Adapters[configuration.ClientClaude].Enabled {
		t.Fatalf("configuration was not rolled back: %#v, %v", stored, loadErr)
	}
}
