package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

type reconciliationDiscovery struct{ result discovery.Result }

func (d reconciliationDiscovery) Discover() discovery.Result { return d.result }

type reconciliationRunner struct{ plans []adapters.ProcessPlan }

func (r *reconciliationRunner) Run(_ context.Context, plan adapters.ProcessPlan) error {
	r.plans = append(r.plans, plan)
	return nil
}

type failingReconciliationRunner struct{ err error }

func (r failingReconciliationRunner) Run(_ context.Context, _ adapters.ProcessPlan) error {
	return r.err
}

func reconciliationConfig(target string) domain.Config {
	cfg := domain.NewConfig()
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "gateway", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

func TestCodexProjectionChangedWhenAdapterIsDisabled(t *testing.T) {
	before := reconciliationConfig(filepath.Join(t.TempDir(), "config.toml"))
	after := cloneConfig(before)
	delete(after.Adapters, domain.ClientCodex)
	if !codexProjectionChanged(before, after) {
		t.Fatal("disabling a previously enabled Codex adapter must reconcile its target removal")
	}
}

func TestCommitConfigAndSyncRestoresTargetRemovedFromAdapter(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("model_provider = \"native\"\nmodel = \"gpt-native\"\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before := reconciliationConfig(target)
	app := &App{
		Config: config.NewStore(filepath.Join(dir, "aigw.toml")),
		Discovery: reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID:          discovery.SurfaceCodexCLIStandalone,
			Authority:   discovery.AuthorityAIGW,
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}}}},
	}
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	runtime, _, err := before.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapters.SyncCodexConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	after := cloneConfig(before)
	delete(after.Adapters, domain.ClientCodex)
	if err := commitConfigAndSync(context.Background(), app, before, after, "test"); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatalf("target was not restored\nwant %q\ngot  %q", original, restored)
	}
	if _, err := os.Stat(target + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar remains after adapter removal: %v", err)
	}
	stored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Adapters[domain.ClientCodex].Enabled {
		t.Fatalf("stored config still enables Codex: %#v", stored.Adapters)
	}
	if strings.Contains(string(restored), "AIGW managed") {
		t.Fatalf("restored config keeps managed content: %s", restored)
	}
}

func TestCommitConfigAndSyncRestoresTargetWhenAuthenticationFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before := domain.NewConfig()
	after := reconciliationConfig(target)
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "test-token"); err != nil {
		t.Fatal(err)
	}
	app := &App{
		Config:  config.NewStore(filepath.Join(dir, "aigw.toml")),
		Secrets: secretStore,
		Runner:  failingReconciliationRunner{err: os.ErrPermission},
		Discovery: reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID:          discovery.SurfaceCodexCLIStandalone,
			Authority:   discovery.AuthorityAIGW,
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}}}},
	}
	err := commitConfigAndSync(context.Background(), app, before, after, "test")
	if err == nil || !strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("commitConfigAndSync() error = %v", err)
	}
	restored, readErr := os.ReadFile(target)
	if readErr != nil || !bytes.Equal(restored, original) {
		t.Fatalf("target after authentication rollback = %q, %v; want %q", restored, readErr, original)
	}
	if _, err := os.Stat(target + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar remains after authentication rollback: %v", err)
	}
}

func TestValidateExplicitCodexTargetRejectsJetBrainsOwnedSurfaces(t *testing.T) {
	root := t.TempDir()
	pycharm := filepath.Join(root, "pycharm.toml")
	air := filepath.Join(root, "air.toml")
	junie := filepath.Join(root, "junie")
	discovered := discovery.Result{Surfaces: []discovery.Surface{
		{ID: discovery.SurfacePyCharmCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: pycharm},
		{ID: discovery.SurfaceAirCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: air, ManualFallbackAllowed: true},
		{ID: discovery.SurfaceJunieCLI, Authority: discovery.AuthorityJetBrainsAI, Executable: junie},
	}}
	for _, target := range []string{pycharm, air, junie} {
		if err := validateExplicitCodexTarget(discovered, target); err == nil {
			t.Fatalf("validateExplicitCodexTarget(%q) unexpectedly succeeded", target)
		}
	}
	if err := validateExplicitCodexTarget(discovered, filepath.Join(root, "standalone.toml")); err != nil {
		t.Fatalf("unknown explicit target was rejected: %v", err)
	}
}

func TestRepairRestoresLegacyAirAndKeepsOnlyStandaloneTarget(t *testing.T) {
	dir := t.TempDir()
	standalone := filepath.Join(dir, "standalone", "config.toml")
	air := filepath.Join(dir, "air", "config.toml")
	for _, path := range []string{standalone, air} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	standaloneOriginal := []byte("model_provider = \"native\"\n")
	airOriginal := []byte("model_provider = \"jetbrains\"\nmodel = \"jb-default\"\n")
	if err := os.WriteFile(standalone, standaloneOriginal, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(air, airOriginal, 0o600); err != nil {
		t.Fatal(err)
	}
	before := reconciliationConfig(air)
	runtime, _, err := before.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapters.SyncCodexConfig(air, runtime); err != nil {
		t.Fatal(err)
	}
	runner := &reconciliationRunner{}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "test-token"); err != nil {
		t.Fatal(err)
	}
	output := new(bytes.Buffer)
	app := &App{
		Config:  config.NewStore(filepath.Join(dir, "aigw.toml")),
		Secrets: secretStore,
		Runner:  runner,
		Out:     output,
		Err:     output,
		Discovery: reconciliationDiscovery{result: discovery.Result{
			CodexExecutable: "/opt/codex",
			Surfaces: []discovery.Surface{
				{ID: discovery.SurfaceCodexCLIStandalone, Authority: discovery.AuthorityAIGW, ConfigPath: standalone, Present: true, AutoManaged: true},
				{ID: discovery.SurfaceAirCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: air, Present: true, ManualFallbackAllowed: true},
			},
		}},
	}
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	if err := Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	targets := after.Adapters[domain.ClientCodex].Targets
	if len(targets) != 1 || targets[0] != standalone {
		t.Fatalf("repair targets = %#v, want only %q", targets, standalone)
	}
	restoredAir, err := os.ReadFile(air)
	if err != nil || !bytes.Equal(restoredAir, airOriginal) {
		t.Fatalf("Air after repair = %q, %v; want %q", restoredAir, err, airOriginal)
	}
	if _, err := os.Stat(air + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("Air sidecar remains after repair: %v", err)
	}
	if len(runner.plans) != 1 {
		t.Fatalf("native authentication plans = %#v", runner.plans)
	}
}
