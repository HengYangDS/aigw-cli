package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"testing"

	claudedesktop "aigw-cli/internal/claude/desktop"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

type claudeDesktopFixture struct {
	adapter claudeDesktopAdapter
	cfg     configuration.Config
	deps    Dependencies
	runtime configuration.Runtime
	paths   claudedesktop.Paths
}

func newClaudeDesktopFixture(t *testing.T) claudeDesktopFixture {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "bin", "Claude")
	aigwExecutable := filepath.Join(root, "bin", "aigw")
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{executable, aigwExecutable} {
		if err := os.WriteFile(path, []byte("fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	library := filepath.Join(root, "Claude-3p", "configLibrary")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test/v1"}}
	cfg.Accounts["other"] = configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{Anthropic: "https://other.test/v1"}}
	cfg.Profiles["alternate"] = configuration.Profile{Label: "Opus 5", Account: "gateway", Model: "claude-opus-5", Protocols: []configuration.EndpointProtocol{configuration.ProtocolAnthropic}}
	cfg.Profiles["manual"] = configuration.Profile{Label: "Manual", Account: "gateway", Model: "manual-model"}
	cfg.Profiles["other-account"] = configuration.Profile{Label: "Other", Account: "other", Model: "other-model", Protocols: []configuration.EndpointProtocol{configuration.ProtocolAnthropic}}
	cfg.Profiles["selected"] = configuration.Profile{Label: "Fable 5.1", Account: "gateway", Model: "claude-fable-5-1", Protocols: []configuration.EndpointProtocol{configuration.ProtocolAnthropic}}
	cfg.SetSelectedProfile(configuration.ClientClaudeDesktop, "selected")
	cfg.SetClientActivation(configuration.ClientClaudeDesktop, true, executable, []string{library})
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "secret"); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaudeDesktop, "")
	if err != nil {
		t.Fatal(err)
	}
	return claudeDesktopFixture{
		adapter: claudeDesktopAdapter{},
		cfg:     cfg,
		deps:    Dependencies{Secrets: store, AIGWExecutable: aigwExecutable},
		runtime: runtime,
		paths:   claudedesktop.PathsForLibrary(library),
	}
}

func (fixture claudeDesktopFixture) apply(t *testing.T) {
	t.Helper()
	if _, err := fixture.adapter.Apply(t.Context(), fixture.deps, configuration.NewConfig(), fixture.cfg); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeDesktopAdapterProjectsSelectedAccountCatalogue(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	plans, err := fixture.adapter.Plan(fixture.deps, configuration.NewConfig(), fixture.cfg)
	if err != nil {
		t.Fatal(err)
	}
	library := filepath.Dir(fixture.paths.Profile)
	if len(plans) != 1 || plans[0].Client != configuration.ClientClaudeDesktop || plans[0].Target != library || plans[0].Action != string(claudedesktop.ActionProject) {
		t.Fatalf("plans = %#v", plans)
	}
	fixture.apply(t)
	profileData, err := os.ReadFile(fixture.paths.Profile)
	if err != nil {
		t.Fatal(err)
	}
	var profile map[string]any
	if err := json.Unmarshal(profileData, &profile); err != nil {
		t.Fatal(err)
	}
	models, ok := profile["inferenceModels"].([]any)
	if !ok || len(models) != 2 {
		t.Fatalf("models = %#v", profile["inferenceModels"])
	}
	for index, want := range []string{"claude-fable-5-1", "claude-opus-5"} {
		model, ok := models[index].(map[string]any)
		if !ok || model["name"] != want {
			t.Fatalf("model %d = %#v, want %q", index, models[index], want)
		}
	}
	arguments, ok := profile["inferenceCredentialHelperArgs"].([]any)
	wantArguments := []any{"credential", configuration.ClientClaudeDesktop, fixture.runtime.CredentialProjectionFingerprint(configuration.ClientClaudeDesktop)}
	if !ok || !slices.Equal(arguments, wantArguments) {
		t.Fatalf("credential arguments = %#v, want %#v", arguments, wantArguments)
	}
	if status := fixture.adapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); !status.Ready {
		t.Fatalf("inspection = %#v", status)
	}
}

func TestClaudeDesktopAdapterWithdrawsOwnedProjection(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	fixture.apply(t)
	disabled := fixture.cfg.Clone()
	fixture.adapter.Withdraw(&disabled)
	if _, exists := disabled.Clients[configuration.ClientClaudeDesktop]; exists {
		t.Fatal("withdrawal retained the Claude Desktop binding")
	}
	if _, err := fixture.adapter.Apply(t.Context(), fixture.deps, fixture.cfg, disabled); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fixture.paths.StandardConfig, fixture.paths.ThirdPartyConfig, fixture.paths.Profile, fixture.paths.Metadata, fixture.paths.State} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("withdrawal left %s: %v", path, err)
		}
	}
}

func TestClaudeDesktopAdapterReceiptRestoresThePreimage(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	receipt, err := fixture.adapter.Apply(t.Context(), fixture.deps, configuration.NewConfig(), fixture.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fixture.paths.StandardConfig, fixture.paths.ThirdPartyConfig, fixture.paths.Profile, fixture.paths.Metadata, fixture.paths.State} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rollback left %s: %v", path, err)
		}
	}
}

func TestClaudeDesktopProjectionChangeDetection(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	if fixture.adapter.ProjectionChanged(fixture.cfg, fixture.cfg.Clone()) {
		t.Fatal("unchanged configuration reported a projection change")
	}
	disabled := fixture.cfg.Clone()
	binding := disabled.Clients[configuration.ClientClaudeDesktop]
	binding.Enabled = false
	disabled.Clients[configuration.ClientClaudeDesktop] = binding
	if !fixture.adapter.ProjectionChanged(fixture.cfg, disabled) {
		t.Fatal("disabled projection was not detected")
	}
	retargeted := fixture.cfg.Clone()
	binding = retargeted.Clients[configuration.ClientClaudeDesktop]
	binding.Targets = []string{filepath.Join(t.TempDir(), "configLibrary")}
	retargeted.Clients[configuration.ClientClaudeDesktop] = binding
	if !fixture.adapter.ProjectionChanged(fixture.cfg, retargeted) {
		t.Fatal("target change was not detected")
	}
	recatalogued := fixture.cfg.Clone()
	recatalogued.Profiles["daily"] = configuration.Profile{Label: "Daily", Account: "gateway", Model: "claude-daily", Protocols: []configuration.EndpointProtocol{configuration.ProtocolAnthropic}}
	if !fixture.adapter.ProjectionChanged(fixture.cfg, recatalogued) {
		t.Fatal("catalogue change was not detected")
	}
}

func TestClaudeDesktopAdapterConvergesOnlyForAnInstalledAuthorizedClient(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "bin", "Claude")
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	library := filepath.Join(root, "Claude-3p", "configLibrary")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test/v1"}}
	cfg.Profiles["selected"] = configuration.Profile{Account: "gateway", Model: "claude-fable-5-1", Protocols: []configuration.EndpointProtocol{configuration.ProtocolAnthropic}}
	cfg.SetSelectedProfile(configuration.ClientClaudeDesktop, "selected")
	cfg.SetClientActivation(configuration.ClientClaudeDesktop, true, "", nil)
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "secret"); err != nil {
		t.Fatal(err)
	}
	adapter := claudeDesktopAdapter{}
	discovered := adapter.Discover(discovery.System{GOOS: goruntime.GOOS, Home: root, ClaudeDesktopApp: executable, ClaudeDesktopLibrary: library})
	if err := adapter.Converge(Dependencies{Secrets: store}, &cfg, discovered); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[configuration.ClientClaudeDesktop]
	if binding.Executable != executable || !slices.Equal(binding.Targets, []string{library}) {
		t.Fatalf("binding = %#v", binding)
	}
}
