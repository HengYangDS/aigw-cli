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

func TestTeamManifestAdmitsDirectDMXAPISolAsSetupAlternative(t *testing.T) {
	team := readFile(t, filepath.Join("..", "..", "manifests", "team.toml"))
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	route, exists := manifest.Routes["dmxapi-gpt-6-sol"]
	if !exists || route.Account != "dmxapi" || route.Model != "gpt-6-sol" || route.UpstreamModelID() != "gpt-6-sol" {
		t.Fatalf("direct DMXAPI Sol Route = %#v, present=%t", route, exists)
	}
	if len(route.Interfaces) != 1 {
		t.Fatalf("direct DMXAPI Sol protocols = %#v", route.Interfaces)
	}
	if _, ok := route.Interfaces[configuration.ProtocolOpenAIResponses]; !ok {
		t.Fatalf("direct DMXAPI Sol lacks Responses: %#v", route.Interfaces)
	}
	if got := manifest.Accounts["dmxapi"].Endpoints.OpenAIResponses; got != "https://www.dmxapi.cn/v1" {
		t.Fatalf("team DMXAPI endpoint = %q, want direct provider", got)
	}
	for _, client := range []string{configuration.ClientCodex, configuration.ClientHermes} {
		recommendation := manifest.Recommendations[client]
		if recommendation.Primary.Route != "ucloud-gpt-6-sol" {
			t.Fatalf("%s primary Route changed: %q", client, recommendation.Primary.Route)
		}
		if len(recommendation.Alternatives) < 3 || recommendation.Alternatives[0].Route != "aihubmix-gpt-6-sol" ||
			recommendation.Alternatives[1].Route != "dmxapi-gpt-6-sol" || recommendation.Alternatives[2].Route != "dmxapi-gpt-6-luna" {
			t.Fatalf("%s setup alternatives changed the existing order: %#v", client, recommendation.Alternatives)
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
		journey.uninstallAndRequireOwnedFilesAbsent()
		return
	}
	if !clientFirst {
		journey.setEnvironment(secrets.EnvironmentKey(account), "team-journey-token")
	}
	for _, client := range plan.clients {
		journey.installClientFixture(client)
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
		recommended := plan.manifest.Routes[plan.manifest.Recommendations[clientID].Primary.Route]
		offered := slices.ContainsFunc(slices.Collect(maps.Values(plan.manifest.Routes)), func(route configuration.Route) bool {
			return route.Account == account && route.Model == recommended.Model
		})
		if offered && selected.Model != recommended.Model {
			t.Fatalf("%s activation lost recommended model %q: %q", clientID, recommended.Model, selected.Model)
		}
		if got := strings.TrimSpace(string(journey.run("credential", clientID, selected.CredentialProjectionFingerprint(clientID)))); got != "team-journey-token" {
			t.Fatalf("environment credential differs for %s", clientID)
		}
	}
	before := readFile(t, journey.config)
	journey.run("sync")
	if !bytes.Equal(before, readFile(t, journey.config)) {
		t.Fatal("repeated team synchronization rewrote configuration")
	}
	journey.uninstallAndRequireOwnedFilesAbsent()
}
