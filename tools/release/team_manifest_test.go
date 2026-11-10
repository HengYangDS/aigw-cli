package main

import (
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/tools/release/identity"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	builtinclients "aigw-cli/internal/clients"

	"go.yaml.in/yaml/v3"
)

type teamManifestJourney struct {
	program  string
	team     []byte
	manifest domain.Manifest
	clients  []string
}

type manualModelSelection struct {
	model    string
	protocol domain.EndpointProtocol
}

func TestNativeTeamManifestJourney(t *testing.T) {
	callerHome := t.TempDir()
	callerConfig := filepath.Join(callerHome, "config.yaml")
	callerBytes := []byte("model: caller-owned-model\n")
	if err := os.WriteFile(callerConfig, callerBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERMES_HOME", callerHome)
	t.Cleanup(func() {
		entries, err := os.ReadDir(callerHome)
		if err != nil || len(entries) != 1 || !bytes.Equal(readFile(t, callerConfig), callerBytes) {
			t.Errorf("native team journey changed the caller's Hermes Home: %v", err)
		}
	})
	plan := newTeamManifestJourney(t)
	for _, account := range append([]string{""}, domain.ManifestProviderIDs(plan.manifest)...) {
		if account == "" {
			t.Run("no-token-no-client", func(t *testing.T) { plan.runProvider(t, account, false) })
			continue
		}
		t.Run(account+"/token-before-client", func(t *testing.T) { plan.runProvider(t, account, false) })
		t.Run(account+"/client-before-token", func(t *testing.T) { plan.runProvider(t, account, true) })
	}
}

func TestTeamManifestRecommendsDmxapiBeforeUcloudAndKeepsNativeEndpoint(t *testing.T) {
	team := readFile(t, filepath.Join("..", "..", "manifests", "team.toml"))
	sol := "gpt-6.1-sol"
	manifest, err := store.Parse(team, builtinclients.Admission())
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"dmxapi", "aihubmix", "ucloud"} {
		route, exists := manifest.Models[account+"-gpt-6.1-sol"]
		if !exists || route.Provider != account || route.Family != "gpt-6.1-sol" || route.UpstreamModelID() != "gpt-6.1-sol" {
			t.Fatalf("%s 6.1 Sol Route = %#v, present=%t", account, route, exists)
		}
		if len(route.Protocols) != 1 {
			t.Fatalf("%s 6.1 Sol protocols = %#v", account, route.Protocols)
		}
		if _, ok := route.Protocols[domain.ProtocolOpenAIResponses]; !ok {
			t.Fatalf("%s 6.1 Sol lacks Responses: %#v", account, route.Protocols)
		}
	}
	if got := manifest.Providers["dmxapi"].Endpoints.OpenAIResponses; got != "https://www.dmxapi.cn/v1" {
		t.Fatalf("team DMXAPI endpoint = %q, want direct provider", got)
	}
	for client, want := range map[string][]string{
		codex.ID:  {"dmxapi-" + sol + "-cdx", "ucloud-" + sol},
		hermes.ID: {"dmxapi-" + sol, "ucloud-" + sol},
	} {
		selections := manifest.Recommendations[client].Selections()
		if len(selections) != len(want) {
			t.Fatalf("%s setup choices = %#v, want %q", client, selections, want)
		}
		for index, selection := range selections {
			if selection.Model != want[index] {
				t.Fatalf("%s setup choice %d = %q, want %q", client, index, selection.Model, want[index])
			}
		}
	}
}

func newTeamManifestJourney(t *testing.T) teamManifestJourney {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	team := readFile(t, filepath.Join(root, "manifests", "team.toml"))
	manifest, err := store.Parse(team, builtinclients.Admission())
	if err != nil {
		t.Fatal(err)
	}
	version, err := identity.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	program, _, _ := nativeReleaseCandidate(t, root, version)
	t.Logf("team candidate version=%s sha256=%x", version, sha256.Sum256(readFile(t, program)))
	clients := slices.Collect(maps.Keys(manifest.Recommendations))
	slices.Sort(clients)
	admitted := builtinclients.Admission().IDs()
	slices.Sort(admitted)
	if !slices.Equal(clients, admitted) {
		t.Fatalf("team recommendations cover %q, want every admitted client %q", clients, admitted)
	}
	return teamManifestJourney{program: program, team: team, manifest: manifest, clients: clients}
}

func (plan teamManifestJourney) runProvider(t *testing.T, account string, clientFirst bool) {
	t.Helper()
	journey := newNativeJourney(t, plan.program, "https://unused.example.test", false)
	journey.setEnvironment("CODEX_HOME", filepath.Join(journey.root, "home", ".codex"))
	journey.setEnvironment("CLAUDE_CONFIG_DIR", filepath.Dir(journey.settings))
	if err := os.WriteFile(journey.manifest, plan.team, 0o600); err != nil {
		t.Fatal(err)
	}
	journey.run("setup", "--from", journey.manifest)
	// No client is enabled yet, which check refuses.
	journey.run("status", "--json")
	journey.requireNoClaudeProjection()
	if account == "" {
		journey.uninstallAndRequireInstallationRemoved()
		return
	}
	if !clientFirst {
		journey.setEnvironment(secret.EnvironmentKey(account), "team-journey-token")
	}
	for _, client := range plan.clients {
		journey.installClientFixture(client)
	}
	hermesPath := journey.clientProjectionPaths(hermes.ID)[0]
	if err := os.MkdirAll(filepath.Dir(hermesPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hermesPath, []byte("model:\n  ollama_num_ctx: 65536\nmcp_servers:\n  retained:\n    command: user-mcp\nproviders:\n  personal:\n    base_url: https://personal.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if clientFirst {
		requireNoActivationBeforeToken(t, journey)
		journey.setEnvironment(secret.EnvironmentKey(account), "team-journey-token")
	}
	selected, err := domain.Merge(domain.NewConfig(builtinclients.Admission()), plan.manifest)
	if err != nil {
		t.Fatal(err)
	}
	selected, err = selected.SelectModelsForConnectedProviders([]string{account}, plan.clients...)
	if err != nil {
		t.Fatal(err)
	}
	manual := plan.manualSelections(t, account, selected)
	beforePreview := readFile(t, journey.config)
	var preview struct {
		Selections map[string]string `json:"selections"`
	}
	if err := json.Unmarshal(journey.run("sync", "--dry-run", "--json"), &preview); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforePreview, readFile(t, journey.config)) {
		t.Fatal("late sync preview changed configuration")
	}
	journey.run("sync")
	actual, err := store.NewStore(journey.config, builtinclients.Admission()).Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range plan.clients {
		route := selected.SelectedModel(client)
		// sync names the model as provider/model, which the selected model's
		// provider and ID make; the configuration keeps its key.
		reference := ""
		if route != "" {
			reference = selected.Models[route].Ref().String()
		}
		if planned, got := preview.Selections[client], actual.SelectedModel(client); planned != reference || got != route {
			t.Fatalf("late sync selected %s Route %q after preview %q, want %q as %q", client, got, planned, route, reference)
		}
		if _, isManual := manual[client]; isManual && actual.Clients[client].Enabled {
			t.Fatalf("sync activated manually selected %s for Account %q", client, account)
		}
	}
	for client, choice := range manual {
		journey.run("use", client, journey.modelName(choice.model), "--protocol", choice.protocol.Name())
		actual, err = store.NewStore(journey.config, builtinclients.Admission()).Load()
		if err != nil {
			t.Fatal(err)
		}
		if got := actual.SelectedModel(client); got != choice.model {
			t.Fatalf("explicit use selected %s Route %q, want %q", client, got, choice.model)
		}
	}
	plan.requireSelectedProvider(t, journey, account)
}

func (plan teamManifestJourney) manualSelections(
	t *testing.T, account string, selected domain.Config,
) map[string]manualModelSelection {
	t.Helper()
	manual := make(map[string]manualModelSelection)
	for _, client := range plan.clients {
		if selected.SelectedModel(client) != "" {
			continue
		}
		choice, ok := plan.manualModel(account, client)
		if !ok {
			t.Fatalf("Account %q has no compatible declared or manual Route for %s", account, client)
		}
		manual[client] = choice
	}
	return manual
}

func (plan teamManifestJourney) manualModel(account, client string) (manualModelSelection, bool) {
	spec, found := builtinclients.Admission().Spec(client)
	if !found {
		return manualModelSelection{}, false
	}
	recommendation, found := plan.manifest.Recommendations[client]
	if !found {
		return manualModelSelection{}, false
	}
	model := plan.manifest.Models[recommendation.Primary.Model].Family
	for _, routeID := range slices.Sorted(maps.Keys(plan.manifest.Models)) {
		route := plan.manifest.Models[routeID]
		if route.Provider != account || route.Family != model {
			continue
		}
		protocols := spec.CompatibleModelProtocols(plan.manifest.Providers[account], route)
		if len(protocols) == 0 {
			continue
		}
		protocol := recommendation.Primary.Protocol
		if !slices.Contains(protocols, protocol) {
			protocol = protocols[0]
		}
		return manualModelSelection{model: routeID, protocol: protocol}, true
	}
	return manualModelSelection{}, false
}

func requireNoActivationBeforeToken(t *testing.T, journey *journeyFixture) {
	t.Helper()
	var preview struct {
		EnabledClients       int               `json:"enabled_clients"`
		Selections           map[string]string `json:"selections"`
		Projections          []json.RawMessage `json:"projections"`
		CredentialEntrypoint *json.RawMessage  `json:"credential_entrypoint"`
	}
	if err := json.Unmarshal(journey.run("sync", "--dry-run", "--json"), &preview); err != nil {
		t.Fatalf("decode client-first sync preview: %v", err)
	}
	if preview.EnabledClients != 0 || len(preview.Selections) != 0 || len(preview.Projections) != 0 || preview.CredentialEntrypoint != nil {
		t.Fatalf("client-first sync planned activation without a Token: %+v", preview)
	}
	journey.run("sync")
	beforeToken, err := store.NewStore(journey.config, builtinclients.Admission()).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeToken.EnabledClientIDs()) != 0 {
		t.Fatalf("client-first sync activated before a Token: %#v", beforeToken.Clients)
	}
	journey.requireNoClaudeProjection()
}

func (plan teamManifestJourney) requireSelectedProvider(t *testing.T, journey *journeyFixture, account string) {
	t.Helper()
	cfg, err := store.NewStore(journey.config, builtinclients.Admission()).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) != len(plan.manifest.Providers) || len(cfg.Models) != len(plan.manifest.Models) {
		t.Fatal("setup lost reviewed team capabilities")
	}
	for _, clientID := range plan.clients {
		selected, err := cfg.ResolveRuntime(clientID, "")
		if err != nil || selected.ProviderID != account || !cfg.Clients[clientID].Enabled {
			t.Fatalf("one connected Account did not activate %s: %#v, %v", clientID, selected, err)
		}
		if got := strings.TrimSpace(string(journey.run("credential", clientID, selected.CredentialProjectionFingerprint(clientID)))); got != "team-journey-token" {
			t.Fatalf("environment credential differs for %s", clientID)
		}
	}
	before := readFile(t, journey.config)
	plan.requireHermesWireCatalog(t, journey, cfg, account)
	journey.run("sync")
	if !bytes.Equal(before, readFile(t, journey.config)) {
		t.Fatal("repeated team synchronization rewrote configuration")
	}
	journey.uninstallAndRequireInstallationRemoved()
}

func (plan teamManifestJourney) requireHermesWireCatalog(t *testing.T, journey *journeyFixture, cfg domain.Config, account string) {
	t.Helper()
	var projected struct {
		Model struct {
			OllamaContext int `yaml:"ollama_num_ctx"`
		} `yaml:"model"`
		MCP       map[string]struct{ Command string } `yaml:"mcp_servers"`
		Providers map[string]struct {
			Models   []string `yaml:"models"`
			Endpoint string   `yaml:"base_url"`
		} `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(t, journey.clientProjectionPaths(hermes.ID)[0]), &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Model.OllamaContext != 65536 || projected.MCP["retained"].Command != "user-mcp" || projected.Providers["personal"].Endpoint != "https://personal.test" {
		t.Fatal("native Hermes projection changed unowned settings")
	}
	spec, found := builtinclients.Admission().Spec(hermes.ID)
	if !found {
		t.Fatal("Hermes client contract is absent")
	}
	expected := make(map[string][]string)
	for _, route := range plan.manifest.Models {
		if route.Provider != account {
			continue
		}
		for _, protocol := range spec.CompatibleModelProtocols(cfg.Providers[account], route) {
			id := "aigw-" + account + "-" + strings.ReplaceAll(string(protocol), "_", "-")
			expected[id] = append(expected[id], route.UpstreamModelID())
		}
	}
	for id, models := range expected {
		slices.Sort(models)
		models = slices.Compact(models)
		actual := slices.Clone(projected.Providers[id].Models)
		slices.Sort(actual)
		if !slices.Equal(actual, models) {
			t.Errorf("native Hermes provider %q wire catalog = %q, want %q", id, actual, models)
		}
	}
}
