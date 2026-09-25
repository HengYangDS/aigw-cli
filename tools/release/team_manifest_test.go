package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"crypto/sha256"
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
		t.Run(account, func(t *testing.T) { plan.runAccount(t, account) })
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
	return teamManifestJourney{program: program, team: team, manifest: manifest, clients: clients}
}

func (plan teamManifestJourney) runAccount(t *testing.T, account string) {
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
	journey.setEnvironment(secrets.EnvironmentKey(account), "team-journey-token")
	for _, client := range plan.clients {
		journey.installClientFixture(client)
	}
	journey.run("sync")
	selected, err := configuration.Merge(configuration.NewConfig(), plan.manifest)
	if err != nil {
		t.Fatal(err)
	}
	selected, err = selected.SelectRoutesForConnectedAccounts([]string{account}, plan.clients...)
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range plan.clients {
		route := selected.SelectedRoute(client)
		if route == "" {
			t.Fatalf("Account %q has no compatible recommended Route for %s", account, client)
		}
		journey.run("use", "--for", client, route)
	}
	plan.requireSelectedAccount(t, journey, account)
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
