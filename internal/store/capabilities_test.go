package store

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

var publishedSchemaSixCapabilities = []domain.Capability{
	domain.CapabilityText, domain.CapabilityReasoning, domain.CapabilityStreaming, domain.CapabilityTools,
	domain.CapabilityStructuredOutput, domain.CapabilityContinuation, domain.CapabilityCompaction, domain.CapabilityMultimodalInput,
}

func TestUnpublishedModelCapabilityStaysOutOfCanonicalConfiguration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	if err := store.Save(toolSearchConfig()); err != nil {
		t.Fatal(err)
	}
	canonical, err := decodeTOMLConfig(readStoreFile(t, store.Path()), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if got := canonical.Models["claude"].Protocols[domain.ProtocolAnthropic]; !slices.Equal(got, publishedSchemaSixCapabilities) {
		t.Fatalf("canonical Anthropic capabilities = %v, want the published schema-six vocabulary", got)
	}
	if got := canonical.Models["gpt"].Protocols[domain.ProtocolOpenAIResponses]; !slices.Equal(got, []domain.Capability{domain.CapabilityText, domain.CapabilityReasoning}) {
		t.Fatalf("canonical Responses capabilities = %v", got)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range append(slices.Clone(publishedSchemaSixCapabilities), domain.CapabilityToolSearch) {
		if !loaded.Models["claude"].Declares(domain.ProtocolAnthropic, capability) {
			t.Fatalf("stored Route lost %q: %v", capability, loaded.Models["claude"].Protocols)
		}
	}
	credential, err := store.LoadCredentialConfig()
	if err != nil {
		t.Fatal(err)
	}
	if credential.Models["claude"].Declares(domain.ProtocolAnthropic, domain.CapabilityToolSearch) {
		t.Fatal("credential configuration read the capability component")
	}
}

func TestCanonicalLocationCapabilityMovesOnTheNextWrite(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	writePredecessorConfiguration(t, store, toolSearchConfig())
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Models["claude"].Declares(domain.ProtocolAnthropic, domain.CapabilityToolSearch) {
		t.Fatal("the earlier canonical location was not accepted")
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(before, loaded); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(readStoreFile(t, store.Path()), []byte(domain.CapabilityToolSearch)) {
		t.Fatal("the next write kept the capability in the canonical configuration")
	}
	if _, err := os.Stat(capabilityComponentPath(store)); err != nil {
		t.Fatalf("capability component was not written: %v", err)
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if equal, err := configurationsEqual(loaded, reloaded); err != nil || !equal {
		t.Fatalf("moving the capability changed the configuration: equal=%v err=%v", equal, err)
	}
}

func TestStoredCapabilityAppliesOnlyToItsRecordedModel(t *testing.T) {
	for _, change := range []string{"none", "upstream model", "endpoint", "account", "interface", "route"} {
		t.Run(change, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			if err := store.Save(toolSearchConfig()); err != nil {
				t.Fatal(err)
			}
			cfg, err := store.LoadCredentialConfig()
			if err != nil {
				t.Fatal(err)
			}
			route := cfg.Models["claude"]
			switch change {
			case "upstream model":
				route.UpstreamModel = "claude-other"
			case "endpoint":
				account := cfg.Providers["ucloud"]
				account.Endpoints.Anthropic = "https://other.test"
				cfg.Providers["ucloud"] = account
			case "account":
				cfg.Providers["other"] = cfg.Providers["ucloud"]
				route.Provider = "other"
			case "interface":
				route.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText}}
			}
			cfg.Models["claude"] = route
			if change == "route" {
				delete(cfg.Models, "claude")
			}
			writePredecessorConfiguration(t, store, cfg)
			loaded, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := loaded.Models["claude"].Declares(domain.ProtocolAnthropic, domain.CapabilityToolSearch), change == "none"; got != want {
				t.Fatalf("stored capability applies = %v, want %v", got, want)
			}
			if err := store.Save(loaded); err != nil {
				t.Fatal(err)
			}
			_, err = os.Stat(capabilityComponentPath(store))
			if retained := err == nil; retained != (change == "none") {
				t.Fatalf("capability component retained = %v after the next write: %v", retained, err)
			}
		})
	}
}

func TestDamagedCapabilityComponentFailsOnlyFullReads(t *testing.T) {
	fixture := toolSearchConfig()
	responses := capabilityIdentity(fixture, fixture.Models["gpt"], domain.ProtocolOpenAIResponses)
	for name, component := range map[string]string{
		"syntax":          "[routes",
		"empty":           "",
		"unknown":         "[routes.claude.anthropic]\nidentity = 'stale'\ncapabilities = ['future_capability']\n",
		"published":       "[routes.claude.anthropic]\nidentity = 'stale'\ncapabilities = ['text']\n",
		"repeated":        "[routes.claude.anthropic]\nidentity = 'stale'\ncapabilities = ['tool_search', 'tool_search']\n",
		"unknown key":     "[routes.claude.anthropic]\nidentity = 'stale'\ncapabilities = ['tool_search']\norder = 1\n",
		"no qualifier":    "[routes.claude.anthropic]\nidentity = 'stale'\ncapabilities = []\n",
		"wrong interface": "[routes.gpt.openai_responses]\nidentity = '" + responses + "'\ncapabilities = ['tool_search']\n",
	} {
		t.Run(name, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			if err := store.Save(toolSearchConfig()); err != nil {
				t.Fatal(err)
			}
			canonical := readStoreFile(t, store.Path())
			if err := os.WriteFile(capabilityComponentPath(store), []byte(component), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(); err == nil {
				t.Fatal("damaged capability component was accepted")
			}
			if _, err := store.LoadCredentialConfig(); err != nil {
				t.Fatalf("credential configuration depends on the capability component: %v", err)
			}
			if !bytes.Equal(readStoreFile(t, store.Path()), canonical) || string(readStoreFile(t, capabilityComponentPath(store))) != component {
				t.Fatal("refused capability read changed stored bytes")
			}
		})
	}
}

func TestStoredCapabilitySurvivesRecovery(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := toolSearchConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientClaude}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(readStoreFile(t, store.Path()+".verified.json"), []byte(domain.CapabilityToolSearch)) {
		t.Fatal("verified checkpoint carries an unpublished capability")
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil || !checkpoint.Config.Models["claude"].Declares(domain.ProtocolAnthropic, domain.CapabilityToolSearch) {
		t.Fatalf("verified checkpoint lost the stored capability: %v", err)
	}
	state, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConvergeVerifiedBackup(state.Snapshot); err != nil {
		t.Fatal(err)
	}
	backup, err := store.LoadBackup()
	if err != nil || !backup.Models["claude"].Declares(domain.ProtocolAnthropic, domain.CapabilityToolSearch) {
		t.Fatalf("verified backup lost the stored capability: %v", err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	route := changed.Models["claude"]
	route.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityText}}
	changed.Models["claude"] = route
	after, err := store.Commit(before, changed)
	if err != nil {
		t.Fatal(err)
	}
	if current, err := store.Load(); err != nil || current.Models["claude"].Declares(domain.ProtocolAnthropic, domain.CapabilityToolSearch) {
		t.Fatalf("committed removal kept the stored capability: %v", err)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Load()
	if err != nil || !restored.Models["claude"].Declares(domain.ProtocolAnthropic, domain.CapabilityToolSearch) {
		t.Fatalf("compensation did not restore the stored capability: %v", err)
	}
	if got, err := store.CaptureSnapshot(); err != nil || !before.equal(got) {
		t.Fatalf("capability compensation changed the preimage: %v", err)
	}
}

// toolSearchConfig declares every published schema-six capability and the
// later tool_search capability on one Anthropic Route.
func toolSearchConfig() domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{
		Anthropic: "https://ucloud.test", OpenAIResponses: "https://ucloud.test/v1",
	}}
	cfg.Models["claude"] = domain.Model{
		Label: "Claude", Provider: "ucloud", Family: "claude", UpstreamModel: "claude-wire",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: append(slices.Clone(publishedSchemaSixCapabilities), domain.CapabilityToolSearch),
		},
	}
	cfg.Models["gpt"] = domain.Model{
		Label: "GPT", Provider: "ucloud", Family: "gpt",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText, domain.CapabilityReasoning}},
	}
	return cfg
}

// writePredecessorConfiguration replaces only the canonical file, as a release
// that knows no capability component writes it.
func writePredecessorConfiguration(t *testing.T, store Store, cfg domain.Config) {
	t.Helper()
	cfg.Normalize()
	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func capabilityComponentPath(store Store) string {
	return strings.TrimSuffix(store.Path(), filepath.Ext(store.Path())) + ".capabilities.toml"
}

func readStoreFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
