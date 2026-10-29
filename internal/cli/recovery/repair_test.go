package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/clients"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/presentation"
)

type staticDiscovery struct{ result discovery.Result }

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: goruntime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

func (candidate staticDiscovery) Discover() discovery.Result { return candidate.result }

func TestRenderRepairPreviewNamesEachClientAndItsSurface(t *testing.T) {
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(), Out: out, RenderOut: out, Width: 120}
	explicit := string(codex.HomeExplicit("0123456789ab"))
	plans := []client.ProjectionPlan{
		{Client: codex.ID, Target: "/known", Action: "update", Surface: string(codex.HomeDefault)},
		{Client: codex.ID, Target: "/explicit", Action: "create", Surface: explicit},
		{Client: claudedesktop.ID, Target: "/library", Action: "update", Surface: string(claudedesktop.LibrarySurface)},
	}
	if err := renderRepairResult(runtime, true, false, true, plans, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Repair preview", "Codex · " + string(codex.HomeDefault), "Codex · " + explicit, "Claude Desktop · " + string(claudedesktop.LibrarySurface), "create"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("output lacks %q: %s", want, out.String())
		}
	}
	if bytes.Contains(out.Bytes(), []byte("claude-settings")) {
		t.Fatalf("output names another client's surface: %s", out.String())
	}
	column := -1
	for line := range strings.SplitSeq(out.String(), "\n") {
		if prefix, _, found := strings.Cut(line, "update"); found {
			if column >= 0 && presentation.DisplayWidth(prefix) != column {
				t.Fatalf("repair preview action columns differ:\n%s", out.String())
			}
			column = presentation.DisplayWidth(prefix)
		}
	}
}

func TestRenderRepairPreviewJSONReportsEachPlannedSurface(t *testing.T) {
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(), Out: out}
	plans := []client.ProjectionPlan{
		{Client: claude.ID, Target: "/settings.json", Action: "update", ChangesState: true, Surface: string(claude.SettingsSurface)},
		{Client: claudedesktop.ID, Target: "/library", Action: "unchanged", Surface: string(claudedesktop.LibrarySurface)},
		{Client: codex.ID, Target: "/known", Action: "update", ChangesState: true, Surface: string(codex.HomeDefault)},
		{Client: hermes.ID, Target: "/hermes/config.yaml", Action: "unchanged", Surface: string(hermes.HomeSurface)},
	}

	if err := renderRepairResult(runtime, true, true, false, plans, nil); err != nil {
		t.Fatal(err)
	}
	var got repairResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.DryRun || got.ConfigurationAction != "already-converged" || got.NextAction != "aigw repair" || len(got.Projections) != len(plans) {
		t.Fatalf("preview = %#v", got)
	}
	for index, plan := range plans {
		if projection := got.Projections[index]; projection.Client != plan.Client || projection.SurfaceID != plan.Surface {
			t.Fatalf("projection %d = %#v, want client %s on surface %s", index, projection, plan.Client, plan.Surface)
		}
	}
}

func TestRunRepairReportsDiscoveryFailureWithoutMutatingConfiguration(t *testing.T) {
	store, cfg := configuredRepairStore(t)
	if err := runRepair(context.Background(), invocation.Context{Clients: clients.Registry(), Config: store}, false, false); err == nil || !strings.Contains(err.Error(), "discovery") {
		t.Fatalf("runRepair() error = %v, want discovery failure", err)
	}
	if saved, err := store.Load(); err != nil || saved.SelectedRoute(codex.ID) != cfg.SelectedRoute(codex.ID) {
		t.Fatalf("configuration changed after discovery failure: cfg=%#v error=%v", saved, err)
	}
}

func configuredRepairStore(t *testing.T) (configuration.Store, configuration.Config) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := configuration.NewConfig(clients.Admission())
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	cfg.Routes["one"] = configuration.Route{
		Label: "One", Account: "one", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(claude.ID, "one", "")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return store, cfg
}

func TestRepairDesiredConfigPreservesExplicitCodexIntentWhenDiscoveryIsEmpty(t *testing.T) {
	before := configuration.NewConfig(clients.Admission())
	before.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	before.Routes["one"] = configuration.Route{
		Label: "One", Account: "one", Model: "gpt",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	before.SetSelectedRoute(codex.ID, "one", "")
	before.SetClientActivation(codex.ID, true, "/old", nil)
	runtime := invocation.Context{Clients: clients.Registry(), Discovery: staticDiscovery{result: discovery.Result{}}}
	after, _, err := invocation.Synchronizer(runtime).DesiredClientConfiguration(before)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Clients[codex.ID]; !reflect.DeepEqual(got, before.Clients[codex.ID]) {
		t.Fatalf("explicit client intent changed: got %#v, want %#v", got, before.Clients[codex.ID])
	}
}

func TestRunRepairReturnsConfigurationCommitFailure(t *testing.T) {
	store, cfg := configuredRepairStore(t)
	backup := store.Path() + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := invocation.Context{Clients: clients.Registry(),
		Config: store,
		Discovery: staticDiscovery{result: discovery.Result{Executables: map[string]string{
			claude.ID: "/opt/claude",
		}}},
		Host: hostAt(t.TempDir()),
	}
	if err := runRepair(context.Background(), runtime, false, false); err == nil {
		t.Fatal("configuration commit failure was accepted")
	}
	got, err := store.Load()
	if err != nil || got.SelectedRoute(codex.ID) != cfg.SelectedRoute(codex.ID) {
		t.Fatalf("configuration changed: %#v, %v", got, err)
	}
}

func TestRunRepairReturnsDryRunPlanAndConvergedProjectionFailures(t *testing.T) {
	tests := []struct {
		name   string
		dryRun bool
	}{
		{name: "dry-run plan", dryRun: true},
		{name: "converged projection"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
			cfg := configuration.NewConfig(clients.Admission())
			cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
			cfg.Routes["one"] = configuration.Route{
				Label: "One", Account: "one", Model: "gpt-test",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
			}
			cfg.SetSelectedRoute(codex.ID, "one", "")
			missingTarget := filepath.Join(t.TempDir(), "missing", "configuration.toml")
			cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{missingTarget})
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			runtime := invocation.Context{Clients: clients.Registry(),
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Config:     store,
				Discovery:  staticDiscovery{result: discovery.Result{}},
				Out:        &bytes.Buffer{},
			}
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if err := runRepair(context.Background(), runtime, test.dryRun, false); err == nil || !strings.Contains(err.Error(), "does not exist") {
				t.Fatalf("repair error = %v", err)
			}
			after, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("failed repair changed configuration, backup, or verified checkpoint")
			}
		})
	}
}

func TestRunRepairReconcilesEveryEnabledAdapterWhenConfigurationIsConverged(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	settingsHome := t.TempDir()
	settingsPath := filepath.Join(settingsHome, ".claude", "settings.json")
	executable := filepath.Join(t.TempDir(), "aigw")
	cfg := configuration.NewConfig(clients.Admission())
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	cfg.Routes["one"] = configuration.Route{
		Label: "One", Account: "one", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(claude.ID, "one", "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(),
		Executable: executable,
		Config:     store,
		Discovery:  staticDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: "/opt/claude"}}},
		Host:       hostAt(settingsHome),
		Out:        out,
	}

	if err := runRepair(context.Background(), runtime, false, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read repaired Claude projection: %v", err)
	}
	var settings struct {
		APIKeyHelper string            `json:"apiKeyHelper"`
		Environment  map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("decode repaired Claude projection: %v", err)
	}
	if settings.Environment["ANTHROPIC_BASE_URL"] != "https://one.test" || !strings.Contains(settings.APIKeyHelper, executable) {
		t.Fatalf("Claude projection = %s", data)
	}
	for _, test := range []struct {
		label      string
		action     string
		nextAction string
	}{
		{"/opt/claude", "already-converged", "aigw check"},
		{"/updated/claude", "already-converged", "aigw check"},
	} {
		out.Reset()
		runtime.Discovery = staticDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: test.label}}}
		if err := runRepair(t.Context(), runtime, true, true); err != nil {
			t.Fatal(err)
		}
		var result repairResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.ConfigurationAction != test.action || result.NextAction != test.nextAction || !result.DryRun {
			t.Fatalf("repair preview = %+v, %v; want action %s and next action %s", result, err, test.action, test.nextAction)
		}
	}
}

func TestRepairPreservesUnprovenCachedCredentialEntrypoint(t *testing.T) {
	store, _ := configuredRepairStore(t)
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	helper := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("AIGW fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := credential.EnsureEntrypoint(source, helper); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(), Executable: source, CredentialPath: helper, Config: store, Discovery: staticDiscovery{}, Out: out}
	if err := runRepair(t.Context(), runtime, true, true); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		CredentialEntrypoint *struct {
			Path   string `json:"path"`
			Action string `json:"action"`
		} `json:"credential_entrypoint"`
		NextAction string `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.CredentialEntrypoint != nil || preview.NextAction != "aigw check" {
		t.Fatalf("repair preview invented cached-command cleanup: %+v", preview)
	}
	if _, err := os.Lstat(helper); err != nil {
		t.Fatalf("repair dry-run removed the entrypoint: %v", err)
	}
	out.Reset()
	if err := runRepair(t.Context(), runtime, true, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Remove "+helper) {
		t.Fatalf("human repair preview proposed unsafe cached-command cleanup: %s", out.String())
	}
	if err := runRepair(t.Context(), runtime, false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(helper); err != nil {
		t.Fatalf("repair removed a possible cached command: %v", err)
	}
}
