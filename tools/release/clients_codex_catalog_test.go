//go:build client_acceptance

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"aigw-cli/internal/adapter/codex"
	codexcatalog "aigw-cli/internal/adapter/codex/catalog"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/secret"

	"github.com/pelletier/go-toml/v2"
)

// A model_catalog_json that the user names relative to the Codex configuration
// stays the user's, and both verification paths reach it from their private
// homes. The fixture is the installed client's own bundled catalogue projected
// for the Route's model, so every Codex version parses it.
func (p nativeClientJourneyPlan) runCodexRelativeCatalog(t *testing.T) {
	t.Helper()
	executable, err := requiredClientInput("AIGW_ACCEPTANCE_CODEX", false)
	if err != nil {
		t.Fatal(err)
	}
	selection := p.manifest.Recommendations[codex.ID].Primary
	route := p.manifest.Routes[selection.Route]
	protocol := selection.Protocol
	if protocol == "" {
		spec, _ := clients.Admission().Spec(codex.ID)
		protocol = spec.EndpointProtocols[0]
	}
	const token = "native-relative-catalog-token"
	var completions atomic.Int64
	server, _ := newNativeClientServer(t, codex.ID, protocol, route.UpstreamModel, token, &completions)
	journey := newNativeJourney(t, p.candidate, server.URL+"/v1", false)
	if err := journey.prepareNativeClient(codex.ID, executable, p.team); err != nil {
		t.Fatal(err)
	}
	_, bundled, err := codex.ReadBundledCatalog(executable)
	if err != nil {
		t.Fatal(err)
	}
	document, err := codexcatalog.Parse(bundled)
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := document.Project(route.UpstreamModel, route.Model)
	if catalog == nil {
		t.Fatalf("the installed Codex catalogue has no entry for %s", route.UpstreamModel)
	}
	configPath := filepath.Join(journey.root, "home", ".codex", "config.toml")
	catalogPath := filepath.Join(filepath.Dir(configPath), "user-catalog.json")
	if err := os.WriteFile(catalogPath, catalog, 0o600); err != nil {
		t.Fatal(err)
	}
	reference := []byte("model_catalog_json = \"user-catalog.json\"\n")
	if err := os.WriteFile(configPath, append(reference, readFile(t, configPath)...), 0o600); err != nil {
		t.Fatal(err)
	}
	journey.setEnvironment(secret.EnvironmentKey(route.Account), token)
	journey.run("setup", "--from", journey.manifest, "--account", route.Account)
	journey.enableNativeClient(codex.ID, executable)
	for _, args := range [][]string{
		{"verify", "--for", codex.ID},
		{"verify", "--for", codex.ID, "--route", selection.Route},
	} {
		before := completions.Load()
		journey.run(args...)
		var document struct {
			Catalog string `toml:"model_catalog_json"`
		}
		if err := toml.Unmarshal(readFile(t, configPath), &document); err != nil {
			t.Fatal(err)
		}
		if completions.Load() == before || document.Catalog != "user-catalog.json" || !bytes.Equal(readFile(t, catalogPath), catalog) {
			t.Fatalf("%v: requests=%d reference=%q; the user's catalogue must stay as written", args, completions.Load()-before, document.Catalog)
		}
	}
	journey.uninstallWithAndRequireInstallationRemoved(p.candidate)
	t.Logf("Codex verification reached the user's relative catalogue on both paths; requests=%d", completions.Load())
}
