package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type teamManifestJourney struct {
	program  string
	team     []byte
	manifest configuration.Manifest
	clients  []string
}

func TestNativeTeamManifestJourney(t *testing.T) {
	plan := newTeamManifestJourney(t)
	for _, account := range append([]string{""}, configuration.ManifestAccountNames(plan.manifest)...) {
		if account == "" {
			t.Run("no-token-no-client", func(t *testing.T) { plan.runAccount(t, account, false) })
			continue
		}
		t.Run(account+"/token-before-client", func(t *testing.T) { plan.runAccount(t, account, false) })
		t.Run(account+"/client-before-token", func(t *testing.T) { plan.runAccount(t, account, true) })
	}
}

func TestTeamManifestRecommendsQualifiedSolAndRetainsAccountFallbacks(t *testing.T) {
	team := readFile(t, filepath.Join("..", "..", "manifests", "team.toml"))
	sol := "gpt-6.1-sol"
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"dmxapi", "aihubmix", "ucloud"} {
		route, exists := manifest.Routes[account+"-gpt-6.1-sol"]
		if !exists || route.Account != account || route.Model != "gpt-6.1-sol" || route.UpstreamModelID() != "gpt-6.1-sol" {
			t.Fatalf("%s 6.1 Sol Route = %#v, present=%t", account, route, exists)
		}
		if len(route.Interfaces) != 1 {
			t.Fatalf("%s 6.1 Sol protocols = %#v", account, route.Interfaces)
		}
		if _, ok := route.Interfaces[configuration.ProtocolOpenAIResponses]; !ok {
			t.Fatalf("%s 6.1 Sol lacks Responses: %#v", account, route.Interfaces)
		}
	}
	if got := manifest.Accounts["dmxapi"].Endpoints.OpenAIResponses; got != "https://www.dmxapi.cn/v1" {
		t.Fatalf("team DMXAPI endpoint = %q, want direct provider", got)
	}
	for client, want := range map[string][]string{
		configuration.ClientCodex:  {"dmxapi-" + sol + "-cdx", "ucloud-" + sol, "aihubmix-" + sol},
		configuration.ClientHermes: {"dmxapi-" + sol, "ucloud-" + sol, "aihubmix-" + sol},
	} {
		selections := manifest.Recommendations[client].Selections()
		if len(selections) != len(want) {
			t.Fatalf("%s setup choices = %#v, want %q", client, selections, want)
		}
		for index, selection := range selections {
			if selection.Route != want[index] {
				t.Fatalf("%s setup choice %d = %q, want %q", client, index, selection.Route, want[index])
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
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	program, _, _ := nativeReleaseCandidate(t, root, version)
	t.Logf("team candidate version=%s sha256=%x", version, sha256.Sum256(readFile(t, program)))
	clients := slices.Collect(maps.Keys(manifest.Recommendations))
	slices.Sort(clients)
	admitted := configuration.AdmittedClientIDs()
	slices.Sort(admitted)
	if !slices.Equal(clients, admitted) {
		t.Fatalf("team recommendations cover %q, want every admitted client %q", clients, admitted)
	}
	return teamManifestJourney{program: program, team: team, manifest: manifest, clients: clients}
}

func (plan teamManifestJourney) runAccount(t *testing.T, account string, clientFirst bool) {
	t.Helper()
	journey := newNativeJourney(t, plan.program, "https://unused.example.test", false)
	journey.setEnvironment("CODEX_HOME", filepath.Join(journey.root, "home", ".codex"))
	journey.setEnvironment("CLAUDE_CONFIG_DIR", filepath.Dir(journey.settings))
	if err := os.WriteFile(journey.manifest, plan.team, 0o600); err != nil {
		t.Fatal(err)
	}
	journey.run("setup", "--from", journey.manifest)
	journey.run("doctor", "--json")
	journey.requireNoClaudeProjection()
	if account == "" {
		journey.uninstallAndRequireInstallationRemoved()
		return
	}
	if !clientFirst {
		journey.setEnvironment(secrets.EnvironmentKey(account), "team-journey-token")
	}
	for _, client := range plan.clients {
		journey.installClientFixture(client)
	}
	hermesPath := journey.clientProjectionPaths(configuration.ClientHermes)[0]
	if err := os.MkdirAll(filepath.Dir(hermesPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hermesPath, []byte("model:\n  ollama_num_ctx: 65536\nmcp_servers:\n  retained:\n    command: user-mcp\nproviders:\n  personal:\n    base_url: https://personal.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if clientFirst {
		requireNoActivationBeforeToken(t, journey)
		journey.setEnvironment(secrets.EnvironmentKey(account), "team-journey-token")
	}
	selected, err := configuration.Merge(configuration.NewConfig(), plan.manifest)
	if err != nil {
		t.Fatal(err)
	}
	selected, err = selected.SelectRoutesForConnectedAccounts([]string{account}, plan.clients...)
	if err != nil {
		t.Fatal(err)
	}
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
	actual, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range plan.clients {
		route := selected.SelectedRoute(client)
		if route == "" {
			t.Fatalf("Account %q has no compatible recommended Route for %s", account, client)
		}
		if planned, got := preview.Selections[client], actual.SelectedRoute(client); planned != route || got != route {
			t.Fatalf("late sync selected %s Route %q after preview %q, want %q", client, got, planned, route)
		}
	}
	plan.requireSelectedAccount(t, journey, account)
}

func requireNoActivationBeforeToken(t *testing.T, journey *journeyFixture) {
	t.Helper()
	var preview struct {
		EnabledClients       int               `json:"enabled_clients"`
		Selections           map[string]string `json:"selections"`
		Targets              []json.RawMessage `json:"targets"`
		CredentialEntrypoint *json.RawMessage  `json:"credential_entrypoint"`
	}
	if err := json.Unmarshal(journey.run("sync", "--dry-run", "--json"), &preview); err != nil {
		t.Fatalf("decode client-first sync preview: %v", err)
	}
	if preview.EnabledClients != 0 || len(preview.Selections) != 0 || len(preview.Targets) != 0 || preview.CredentialEntrypoint != nil {
		t.Fatalf("client-first sync planned activation without a Token: %+v", preview)
	}
	journey.run("sync")
	beforeToken, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeToken.EnabledClientIDs()) != 0 {
		t.Fatalf("client-first sync activated before a Token: %#v", beforeToken.Clients)
	}
	journey.requireNoClaudeProjection()
}

func (plan teamManifestJourney) requireSelectedAccount(t *testing.T, journey *journeyFixture, account string) {
	t.Helper()
	cfg, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != len(plan.manifest.Accounts) || len(cfg.Routes) != len(plan.manifest.Routes) {
		t.Fatal("setup lost reviewed team capabilities")
	}
	for _, clientID := range plan.clients {
		selected, err := cfg.ResolveRuntime(clientID, "")
		if err != nil || selected.AccountID != account || !cfg.Clients[clientID].Enabled {
			t.Fatalf("one connected Account did not activate %s: %#v, %v", clientID, selected, err)
		}
		if got := strings.TrimSpace(string(journey.run("credential", clientID, selected.CredentialProjectionFingerprint(clientID)))); got != "team-journey-token" {
			t.Fatalf("environment credential differs for %s", clientID)
		}
	}
	before := readFile(t, journey.config)
	plan.requireHermesWireCatalogue(t, journey, cfg, account)
	journey.run("sync")
	if !bytes.Equal(before, readFile(t, journey.config)) {
		t.Fatal("repeated team synchronization rewrote configuration")
	}
	journey.uninstallAndRequireInstallationRemoved()
}

func (plan teamManifestJourney) requireHermesWireCatalogue(t *testing.T, journey *journeyFixture, cfg configuration.Config, account string) {
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
	if err := yaml.Unmarshal(readFile(t, journey.clientProjectionPaths(configuration.ClientHermes)[0]), &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Model.OllamaContext != 65536 || projected.MCP["retained"].Command != "user-mcp" || projected.Providers["personal"].Endpoint != "https://personal.test" {
		t.Fatal("native Hermes projection changed unowned settings")
	}
	spec, found := configuration.ClientSpecFor(configuration.ClientHermes)
	if !found {
		t.Fatal("Hermes client contract is absent")
	}
	expected := make(map[string][]string)
	for _, route := range plan.manifest.Routes {
		if route.Account != account {
			continue
		}
		for _, protocol := range spec.CompatibleRouteProtocols(cfg.Accounts[account], route) {
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
			t.Errorf("native Hermes provider %q wire catalogue = %q, want %q", id, actual, models)
		}
	}
}
