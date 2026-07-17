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

type airRouteHarness struct {
	app         *App
	runner      *reconciliationRunner
	secrets     *secrets.MemoryStore
	standalone  string
	air         string
	airOriginal []byte
}

func newAirRouteHarness(t *testing.T) airRouteHarness {
	t.Helper()
	root := t.TempDir()
	standalone := filepath.Join(root, "standalone", "config.toml")
	air := filepath.Join(root, "air", "config.toml")
	for path, body := range map[string][]byte{
		standalone: []byte("model_provider = \"native\"\n"),
		air:        []byte("model_provider = \"jetbrains\"\nmodel = \"jb-default\"\n\n"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	airOriginal, err := os.ReadFile(air)
	if err != nil {
		t.Fatal(err)
	}
	cfg := reconciliationConfig(standalone)
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapters.SyncCodexConfig(standalone, runtime); err != nil {
		t.Fatal(err)
	}
	runner := &reconciliationRunner{}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "secret"); err != nil {
		t.Fatal(err)
	}
	output := new(bytes.Buffer)
	app := &App{
		Config:  config.NewStore(filepath.Join(root, "aigw.toml")),
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
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return airRouteHarness{app: app, runner: runner, secrets: secretStore, standalone: standalone, air: air, airOriginal: airOriginal}
}

func TestAirFallbackDryRunIsReadOnly(t *testing.T) {
	h := newAirRouteHarness(t)
	configBefore, err := os.ReadFile(h.app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	airBefore, err := os.ReadFile(h.air)
	if err != nil {
		t.Fatal(err)
	}
	tokenBefore, err := h.secrets.Get("gateway")
	if err != nil {
		t.Fatal(err)
	}
	if err := Execute(h.app, []string{"route", "fallback", "air", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	configAfter, _ := os.ReadFile(h.app.Config.Path())
	airAfter, _ := os.ReadFile(h.air)
	tokenAfter, _ := h.secrets.Get("gateway")
	if !bytes.Equal(configBefore, configAfter) || !bytes.Equal(airBefore, airAfter) || tokenBefore != tokenAfter {
		t.Fatal("Air fallback dry-run mutated persistent state")
	}
	if _, err := os.Stat(h.air + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("Air sidecar created during dry-run: %v", err)
	}
	if len(h.runner.plans) != 0 {
		t.Fatalf("dry-run executed native authentication: %#v", h.runner.plans)
	}
	output := h.app.Out.(*bytes.Buffer).String()
	if !strings.Contains(output, `"projection_mode": "namespaced-fallback"`) || strings.Contains(output, h.air) {
		t.Fatalf("unsafe dry-run JSON: %s", output)
	}
}

func TestAirFallbackRequiresHostIdleConfirmation(t *testing.T) {
	h := newAirRouteHarness(t)
	err := Execute(h.app, []string{"route", "fallback", "air"})
	if err == nil || !strings.Contains(err.Error(), "--confirm-host-idle") {
		t.Fatalf("error = %v", err)
	}
}

func TestAirFallbackStagesNamespaceAndBindsOnlyAir(t *testing.T) {
	h := newAirRouteHarness(t)
	if err := Execute(h.app, []string{"route", "fallback", "air", "--confirm-host-idle"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := h.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(cfg.Adapters[domain.ClientCodex].Targets, h.air) {
		t.Fatalf("targets = %#v", cfg.Adapters[domain.ClientCodex].Targets)
	}
	if len(h.runner.plans) != 1 || routeEnvValue(h.runner.plans[0].Env, "CODEX_HOME") != filepath.Dir(h.air) {
		t.Fatalf("native auth plans = %#v", h.runner.plans)
	}
	staged, err := os.ReadFile(h.air)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(staged, h.airOriginal) || strings.Contains(string(staged[:len(h.airOriginal)]), `model_provider = "aigw"`) || !strings.Contains(string(staged), "# >>> AIGW fallback provider >>>") {
		t.Fatalf("Air staging changed the active selection:\n%s", staged)
	}
}

func TestAirRestoreRemovesFallbackWithoutDeletingCredential(t *testing.T) {
	h := newAirRouteHarness(t)
	if err := Execute(h.app, []string{"route", "fallback", "air", "--confirm-host-idle"}); err != nil {
		t.Fatal(err)
	}
	plansBeforeRestore := len(h.runner.plans)
	if err := Execute(h.app, []string{"route", "restore", "air", "--confirm-host-idle"}); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(h.air)
	if err != nil || !bytes.Equal(restored, h.airOriginal) {
		t.Fatalf("Air after restore = %q, %v", restored, err)
	}
	if len(h.runner.plans) != plansBeforeRestore {
		t.Fatalf("restore ran native authentication: %#v", h.runner.plans)
	}
	token, err := h.secrets.Get("gateway")
	if err != nil || token != "secret" {
		t.Fatalf("credential changed: %q, %v", token, err)
	}
}

func TestAirRestoreRepairsUnlistedStaleFallback(t *testing.T) {
	h := newAirRouteHarness(t)
	discovered, err := discoveredResult(h.app)
	if err != nil {
		t.Fatal(err)
	}
	airRefs, err := codexTargetRefs(discovered, []string{h.air}, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := h.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapters.ReconcileCodexConfigs(nil, airRefs, runtime); err != nil {
		t.Fatal(err)
	}
	if err := Execute(h.app, []string{"route", "restore", "air", "--confirm-host-idle"}); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(h.air)
	if err != nil || !bytes.Equal(restored, h.airOriginal) {
		t.Fatalf("stale Air fallback was not restored: %q, %v", restored, err)
	}
}

func TestRouteFallbackDryRunsDoNotTakeMutationLock(t *testing.T) {
	if mutationCommand(&App{}, []string{"route", "fallback", "air", "--dry-run"}) {
		t.Fatal("fallback dry-run unexpectedly takes a mutation lock")
	}
	if mutationCommand(&App{}, []string{"route", "restore", "air", "--dry-run"}) {
		t.Fatal("restore dry-run unexpectedly takes a mutation lock")
	}
	if mutationCommand(&App{}, []string{"route", "fallback", "air", "--dry-run=true"}) {
		t.Fatal("fallback --dry-run=true unexpectedly takes a mutation lock")
	}
	if !mutationCommand(&App{}, []string{"route", "fallback", "air", "--confirm-host-idle"}) {
		t.Fatal("fallback apply must take a mutation lock")
	}
}

func routeEnvValue(values []string, key string) string {
	prefix := key + "="
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
}

func _routeFallbackContextCompileGuard(_ context.Context) {}
