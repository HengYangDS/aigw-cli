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
		profile := selected.SelectedRoute(client)
		if profile == "" {
			t.Fatalf("Account %q has no compatible recommended Profile for %s", account, client)
		}
		journey.run("use", "--for", client, profile)
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
		offered := slices.ContainsFunc(slices.Collect(maps.Values(plan.manifest.Routes)), func(profile configuration.Route) bool {
			return profile.Account == account && profile.Model == recommended.Model
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
