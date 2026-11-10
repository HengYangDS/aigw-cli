package claudedesktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"

	"aigw-cli/internal/store"
)

type claudeDesktopFixture struct {
	clientAdapter claudeDesktopAdapter
	cfg           domain.Config
	deps          adapter.Dependencies
	runtime       domain.Runtime
	paths         Paths
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
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test/v1"}}
	cfg.Providers["other"] = domain.Provider{Label: "Other", Endpoints: domain.Endpoints{Anthropic: "https://other.test/v1"}}
	cfg.Models["alternate"] = domain.Model{Label: "Opus 5", Provider: "gateway", Family: "claude-opus-5", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["manual"] = domain.Model{Label: "Manual", Provider: "gateway", Family: "manual-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["other-account"] = domain.Model{Label: "Other", Provider: "other", Family: "other-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["selected"] = domain.Model{Label: "Fable 5.1", Provider: "gateway", Family: "claude-fable-5-1", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(ID, "selected", "")
	cfg.SetClientActivation(ID, true, executable, []string{library})
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "secret"); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	return claudeDesktopFixture{
		clientAdapter: claudeDesktopAdapter{},
		cfg:           cfg,
		deps:          adapter.Dependencies{Secrets: store, AIGWExecutable: aigwExecutable},
		runtime:       runtime,
		paths:         PathsForLibrary(library),
	}
}

func TestLibrarySurfaceIdentityIsStable(t *testing.T) {
	if got := string(LibrarySurface); got != "claude-desktop-config-library" {
		t.Fatalf("Claude Desktop library surface = %q", got)
	}
}

func TestClaudeDesktopCatalogDerivesUnlabeledAlternative(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	route := fixture.cfg.Models["alternate"]
	route.Label = ""
	fixture.cfg.Models["alternate"] = route
	fixture.cfg.Families["claude-opus-5"] = domain.Family{Label: "Claude Opus 5"}
	models := claudeDesktopModels(fixture.cfg, fixture.runtime)
	for _, model := range models {
		if model.Name == "claude-opus-5" {
			if model.Label != "Gateway · Claude Opus 5" {
				t.Fatalf("derived Claude Desktop label = %q", model.Label)
			}
			return
		}
	}
	t.Fatal("Claude Desktop alternative was omitted")
}

func TestClaudeDesktopCatalogDeduplicatesFamiliesButUsesWireNames(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	selected := fixture.cfg.Models["selected"]
	selected.Label = "Sonnet 5.5 CC"
	selected.Family, selected.UpstreamModel = "claude-sonnet-5-5", "claude-sonnet-5-5-cc"
	fixture.cfg.Models["selected"] = selected
	fixture.cfg.Models["ordinary"] = domain.Model{
		Provider: "gateway", Family: "claude-sonnet-5-5", UpstreamModel: "claude-sonnet-5-5",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	alternate := fixture.cfg.Models["alternate"]
	alternate.UpstreamModel = "claude-opus-5-ssvip"
	fixture.cfg.Models["alternate"] = alternate
	runtime, err := fixture.cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	models := claudeDesktopModels(fixture.cfg, runtime)
	if len(models) != 3 || models[0].Name != "claude-sonnet-5-5-cc" || models[1].Name != "claude-opus-5-ssvip" || models[2].Name != "manual-model" {
		t.Fatalf("Claude Desktop model projection = %#v", models)
	}
}

func TestClaudeDesktopShippedChannelDoesNotDuplicateItsFamily(t *testing.T) {
	team, err := os.ReadFile(filepath.Join("..", "..", "..", "manifests", "team.toml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := store.Parse(team, builtInAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(builtInAdmission()), manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetSelectedModel(ID, "dmxapi-claude-sonnet-5-5-cc", "")
	runtime, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	models := claudeDesktopModels(cfg, runtime)
	spec := New().Spec()
	wires := map[string]string{}
	want := map[string]bool{}
	for _, route := range cfg.Models {
		if route.Provider == runtime.ProviderID && slices.Contains(spec.CompatibleModelProtocols(cfg.Providers[route.Provider], route), runtime.Protocol) {
			wires[route.UpstreamModelID()] = route.Family
			want[route.Family] = true
		}
	}
	if len(models) != len(want) || models[0].Name != runtime.UpstreamModel {
		t.Fatalf("shipped Claude Desktop model projection = %#v", models)
	}
	for _, model := range models {
		canonical, admitted := wires[model.Name]
		if !admitted || !want[canonical] {
			t.Fatalf("shipped Claude Desktop projected an ineligible or duplicate logical Model: %#v", models)
		}
		delete(want, canonical)
	}
}

func (fixture claudeDesktopFixture) apply(t *testing.T) {
	t.Helper()
	if _, err := fixture.clientAdapter.Apply(t.Context(), fixture.deps, domain.NewConfig(testAdmission()), fixture.cfg); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeDesktopAdapterProjectsSelectedProviderCatalog(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	plans, err := fixture.clientAdapter.Plan(fixture.deps, domain.NewConfig(testAdmission()), fixture.cfg)
	if err != nil {
		t.Fatal(err)
	}
	library := filepath.Dir(fixture.paths.Profile)
	if len(plans) != 1 || plans[0].Client != ID || plans[0].Target != library || plans[0].Action != string(ActionProject) {
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
	if !ok || len(models) != 3 {
		t.Fatalf("models = %#v", profile["inferenceModels"])
	}
	for index, want := range []string{"claude-fable-5-1", "claude-opus-5", "manual-model"} {
		model, ok := models[index].(map[string]any)
		if !ok || model["name"] != want {
			t.Fatalf("model %d = %#v, want %q", index, models[index], want)
		}
	}
	arguments, ok := profile["inferenceCredentialHelperArgs"].([]any)
	wantArguments := []any{"credential", ID, fixture.runtime.CredentialProjectionFingerprint(ID)}
	if !ok || !slices.Equal(arguments, wantArguments) {
		t.Fatalf("credential arguments = %#v, want %#v", arguments, wantArguments)
	}
	if status := fixture.clientAdapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); !status.Ready {
		t.Fatalf("inspection = %#v", status)
	}
}

func TestClaudeDesktopAdapterWithdrawsOwnedProjection(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	fixture.apply(t)
	disabled := fixture.cfg.Clone()
	fixture.clientAdapter.Withdraw(&disabled)
	if _, exists := disabled.Clients[ID]; exists {
		t.Fatal("withdrawal retained the Claude Desktop binding")
	}
	if _, err := fixture.clientAdapter.Apply(t.Context(), fixture.deps, fixture.cfg, disabled); err != nil {
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
	receipt, err := fixture.clientAdapter.Apply(t.Context(), fixture.deps, domain.NewConfig(testAdmission()), fixture.cfg)
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
	if fixture.clientAdapter.ProjectionChanged(fixture.cfg, fixture.cfg.Clone()) {
		t.Fatal("unchanged configuration reported a projection change")
	}
	disabled := fixture.cfg.Clone()
	binding := disabled.Clients[ID]
	binding.Enabled = false
	disabled.Clients[ID] = binding
	if !fixture.clientAdapter.ProjectionChanged(fixture.cfg, disabled) {
		t.Fatal("disabled projection was not detected")
	}
	retargeted := fixture.cfg.Clone()
	binding = retargeted.Clients[ID]
	binding.Targets = []string{filepath.Join(t.TempDir(), "configLibrary")}
	retargeted.Clients[ID] = binding
	if !fixture.clientAdapter.ProjectionChanged(fixture.cfg, retargeted) {
		t.Fatal("target change was not detected")
	}
	recataloged := fixture.cfg.Clone()
	recataloged.Models["daily"] = domain.Model{Label: "Daily", Provider: "gateway", Family: "claude-daily", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	if !fixture.clientAdapter.ProjectionChanged(fixture.cfg, recataloged) {
		t.Fatal("catalog change was not detected")
	}
}

func TestClaudeDesktopAdapterConvergesOnlyForAnInstalledAuthorizedClient(t *testing.T) {
	goos := goruntime.GOOS
	if goos == "linux" {
		goos = "darwin"
	}
	if goos != "darwin" && goos != "windows" {
		t.Skip("unsupported test host")
	}
	root := t.TempDir()
	env := map[string]string{"HOME": root, "LOCALAPPDATA": root}
	executable := ApplicationPathsFor(goos, env)[0]
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	library, err := LibraryPathFor(goos, env)
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test/v1"}}
	cfg.Models["selected"] = domain.Model{Provider: "gateway", Family: "claude-fable-5-1", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(ID, "selected", "")
	cfg.SetClientActivation(ID, true, "", nil)
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "secret"); err != nil {
		t.Fatal(err)
	}
	clientAdapter := claudeDesktopAdapter{}
	discovered := clientAdapter.Discover(discovery.System{GOOS: goos, Home: root, Env: map[string]string{"LOCALAPPDATA": root}})
	if err := clientAdapter.Converge(adapter.Dependencies{Secrets: store}, &cfg, discovered); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ID]
	if binding.Executable != executable || !slices.Equal(binding.Targets, []string{library}) {
		t.Fatalf("binding = %#v", binding)
	}
}

func TestClaudeDesktopInspectionKeepsIntactRetainedVersionedReader(t *testing.T) {
	retained, current := versionedReaders(t)
	fixture := newClaudeDesktopFixture(t)
	fixture.deps.AIGWExecutable = current
	if status := fixture.clientAdapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); status.Ready || !strings.Contains(status.Issue, "projection differs") {
		t.Fatalf("unsynchronized Claude Desktop projection = %#v", status)
	}
	fixture.deps.AIGWExecutable = retained
	fixture.apply(t)
	before, err := os.ReadFile(fixture.paths.Profile)
	if err != nil {
		t.Fatal(err)
	}
	fixture.deps.AIGWExecutable = current
	if status := fixture.clientAdapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); !status.Ready {
		t.Fatalf("intact retained Claude Desktop reader was rejected: %s", status.Issue)
	}
	after, err := os.ReadFile(fixture.paths.Profile)
	if err != nil || !slices.Equal(before, after) {
		t.Fatalf("inspection changed Claude Desktop profile: %v", err)
	}
	if err := os.Remove(fixture.paths.Profile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.paths.Profile, 0o700); err != nil {
		t.Fatal(err)
	}
	if status := fixture.clientAdapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); status.Ready || strings.Contains(status.Issue, fixture.paths.Profile) {
		t.Fatalf("Claude Desktop inspection exposed an unreadable profile: %#v", status)
	}
	if err := os.Remove(fixture.paths.Profile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.paths.Profile, before, 0o600); err != nil {
		t.Fatal(err)
	}
	changed := fixture.cfg.Clone()
	account := changed.Providers["gateway"]
	account.Endpoints.Anthropic = "https://changed.test"
	changed.Providers["gateway"] = account
	changedRuntime, err := changed.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if status := fixture.clientAdapter.Inspect(t.Context(), fixture.deps, changed, changedRuntime); status.Ready {
		t.Fatal("an endpoint change was mistaken for a retained-reader transition")
	}
	if err := os.RemoveAll(filepath.Dir(retained)); err != nil {
		t.Fatal(err)
	}
	if status := fixture.clientAdapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); status.Ready {
		t.Fatal("missing retained Claude Desktop reader was accepted")
	}
}

func versionedReaders(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, 2)
	for _, version := range []string{"predecessor", "successor"} {
		source := filepath.Join(root, version)
		if err := os.WriteFile(source, []byte(version), 0o700); err != nil {
			t.Fatal(err)
		}
		reader, err := secret.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := secret.EnsureEntrypoint(source, reader); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, reader)
	}
	return paths[0], paths[1]
}
