package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
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

func TestAirRecoverDryRunIsReadOnly(t *testing.T) {
	h := newAirRouteHarness(t)
	stageStaleAirFullSelection(t, h)
	configBefore, err := os.ReadFile(h.app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	airBefore, err := os.ReadFile(h.air)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(h.air + ".aigw-state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Execute(h.app, []string{"route", "recover", "air", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	configAfter, _ := os.ReadFile(h.app.Config.Path())
	airAfter, _ := os.ReadFile(h.air)
	stateAfter, _ := os.ReadFile(h.air + ".aigw-state.json")
	if !bytes.Equal(configAfter, configBefore) || !bytes.Equal(airAfter, airBefore) || !bytes.Equal(stateAfter, stateBefore) {
		t.Fatal("Air recover dry-run mutated persistent state")
	}
	if len(h.runner.plans) != 0 {
		t.Fatalf("Air recover dry-run executed native authentication: %#v", h.runner.plans)
	}
	output := h.app.Out.(*bytes.Buffer).String()
	for _, want := range []string{`"projection_mode": "none"`, `"action": "recover-stale-full-selection"`, `"configuration_action": "remove-air-fallback-target"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("Air recover preview missing %q:\n%s", want, output)
		}
	}
	for _, forbidden := range []string{h.air, "gateway.test", "secret", "model_provider"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("Air recover preview exposed %q:\n%s", forbidden, output)
		}
	}
}

func TestAirRecoverAlreadyExternalDryRunIsPathFreeJSON(t *testing.T) {
	h := newAirRouteHarness(t)
	configBefore, err := os.ReadFile(h.app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	airBefore, err := os.ReadFile(h.air)
	if err != nil {
		t.Fatal(err)
	}
	if err := Execute(h.app, []string{"route", "recover", "air", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	var preview routeChangePreview
	output := h.app.Out.(*bytes.Buffer).Bytes()
	if err := json.Unmarshal(output, &preview); err != nil {
		t.Fatalf("already-external preview is not JSON: %v\n%s", err, output)
	}
	if preview.Action != "already-external" || preview.ConfigurationAction != "already-without-air-fallback-target" {
		t.Fatalf("preview = %#v", preview)
	}
	if bytes.Contains(output, []byte(h.air)) {
		t.Fatalf("already-external preview exposed Air path:\n%s", output)
	}
	configAfter, _ := os.ReadFile(h.app.Config.Path())
	airAfter, _ := os.ReadFile(h.air)
	if !bytes.Equal(configAfter, configBefore) || !bytes.Equal(airAfter, airBefore) {
		t.Fatal("already-external recovery preview mutated persistent state")
	}
}

func TestAirRecoverRejectedStateDoesNotExposePath(t *testing.T) {
	h := newAirRouteHarness(t)
	unsafe := "model_provider = \"aigw\" # managed by AIGW\n"
	if err := os.WriteFile(h.air, []byte(unsafe), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Execute(h.app, []string{"route", "recover", "air", "--dry-run", "--json"})
	if err == nil {
		t.Fatal("unsafe Air recovery unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), h.air) || strings.Contains(h.app.Out.(*bytes.Buffer).String(), h.air) {
		t.Fatalf("rejected Air recovery exposed its config path: %v\n%s", err, h.app.Out.(*bytes.Buffer).String())
	}
}

func TestAirRecoverRequiresHostIdleConfirmation(t *testing.T) {
	h := newAirRouteHarness(t)
	stageStaleAirFullSelection(t, h)
	err := Execute(h.app, []string{"route", "recover", "air"})
	if err == nil || !strings.Contains(err.Error(), "--confirm-host-idle") {
		t.Fatalf("error = %v", err)
	}
}

func TestAirRecoverRemovesStaleFullSelectionAndTarget(t *testing.T) {
	h := newAirRouteHarness(t)
	stageStaleAirFullSelection(t, h)
	if err := Execute(h.app, []string{"route", "recover", "air", "--confirm-host-idle"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := h.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if contains(cfg.Adapters[domain.ClientCodex].Targets, h.air) {
		t.Fatalf("Air remains in AIGW targets: %#v", cfg.Adapters[domain.ClientCodex].Targets)
	}
	recovered, err := os.ReadFile(h.air)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`model_provider = "aigw"`, `# managed by AIGW`, "AIGW managed provider", "[model_providers.aigw]"} {
		if strings.Contains(string(recovered), forbidden) {
			t.Fatalf("recovered Air config retains %q:\n%s", forbidden, recovered)
		}
	}
	if _, err := os.Stat(h.air + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("stale Air sidecar remains after recover: %v", err)
	}
	if len(h.runner.plans) != 0 {
		t.Fatalf("Air recovery executed native authentication: %#v", h.runner.plans)
	}
}

func TestAirRecoverRestoresControlConfigWhenAdapterRecoveryFails(t *testing.T) {
	h := newAirRouteHarness(t)
	stageStaleAirFullSelection(t, h)
	configBefore, err := os.ReadFile(h.app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	originalReconcile := reconcileCodexConfigs
	defer func() { reconcileCodexConfigs = originalReconcile }()
	reconcileCodexConfigs = func([]adapters.CodexTargetRef, []adapters.CodexTargetRef, domain.Runtime) (adapters.CodexReconciliationReceipt, error) {
		return adapters.CodexReconciliationReceipt{}, errors.New("injected Air recovery failure")
	}
	err = Execute(h.app, []string{"route", "recover", "air", "--confirm-host-idle"})
	if err == nil || !strings.Contains(err.Error(), "injected Air recovery failure") {
		t.Fatalf("error = %v", err)
	}
	configAfter, err := os.ReadFile(h.app.Config.Path())
	if err != nil || !bytes.Equal(configAfter, configBefore) {
		t.Fatalf("control-plane config after rollback = %q, %v", configAfter, err)
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
	if mutationCommand(&App{}, []string{"route", "recover", "air", "--dry-run"}) {
		t.Fatal("recover dry-run unexpectedly takes a mutation lock")
	}
	if !mutationCommand(&App{}, []string{"route", "recover", "air", "--confirm-host-idle"}) {
		t.Fatal("recover apply must take a mutation lock")
	}
}

func stageStaleAirFullSelection(t *testing.T, h airRouteHarness) {
	t.Helper()
	cfg, err := h.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := cfg.Adapters[domain.ClientCodex]
	adapter.Targets = addSortedTarget(adapter.Targets, h.air)
	cfg.Adapters[domain.ClientCodex] = adapter
	if err := h.app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	full := "model_provider = \"aigw\" # managed by AIGW\n" +
		"model = \"gpt-test\" # managed by AIGW\n\n" +
		"# >>> AIGW managed provider >>>\n" +
		"[model_providers.aigw]\n" +
		"name = \"AIGW: GPT\"\n" +
		"base_url = \"https://gateway.test/v1\"\n" +
		"wire_api = \"responses\"\n" +
		"requires_openai_auth = true\n" +
		"# <<< AIGW managed provider <<<\n"
	if err := os.WriteFile(h.air, []byte(full), 0o600); err != nil {
		t.Fatal(err)
	}
	fallback := "# >>> AIGW fallback provider >>>\n" +
		"[model_providers.aigw_fallback]\n" +
		"name = \"AIGW fallback: GPT\"\n" +
		"base_url = \"https://gateway.test/v1\"\n" +
		"wire_api = \"responses\"\n" +
		"requires_openai_auth = true\n" +
		"# <<< AIGW fallback provider <<<\n"
	fallbackHash := sha256.Sum256([]byte(fallback))
	state, err := json.Marshal(map[string]string{
		"managed_block_hash": fmt.Sprintf("%x", fallbackHash),
		"projection_mode":    "namespaced-fallback",
		"writer_id":          "aigw-cli",
		"transaction_id":     "stale-air-full-selection",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.air+".aigw-state.json", append(state, '\n'), 0o600); err != nil {
		t.Fatal(err)
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
