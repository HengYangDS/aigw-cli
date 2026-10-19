package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

var publishedSchemaSixCapabilities = []Capability{
	CapabilityText, CapabilityReasoning, CapabilityStreaming, CapabilityTools,
	CapabilityStructuredOutput, CapabilityContinuation, CapabilityCompaction, CapabilityMultimodalInput,
}

func TestUnpublishedRouteCapabilityStaysOutOfCanonicalConfiguration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	if err := store.Save(toolSearchConfig()); err != nil {
		t.Fatal(err)
	}
	canonical, err := decodeTOMLConfig(readStoreFile(t, store.Path()))
	if err != nil {
		t.Fatal(err)
	}
	if got := canonical.Routes["claude"].Interfaces[ProtocolAnthropic]; !slices.Equal(got, publishedSchemaSixCapabilities) {
		t.Fatalf("canonical Anthropic capabilities = %v, want the published schema-six vocabulary", got)
	}
	if got := canonical.Routes["gpt"].Interfaces[ProtocolOpenAIResponses]; !slices.Equal(got, []Capability{CapabilityText, CapabilityReasoning}) {
		t.Fatalf("canonical Responses capabilities = %v", got)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range append(slices.Clone(publishedSchemaSixCapabilities), CapabilityToolSearch) {
		if !loaded.Routes["claude"].Declares(ProtocolAnthropic, capability) {
			t.Fatalf("stored Route lost %q: %v", capability, loaded.Routes["claude"].Interfaces)
		}
	}
	credential, err := store.LoadCredentialConfig()
	if err != nil {
		t.Fatal(err)
	}
	if credential.Routes["claude"].Declares(ProtocolAnthropic, CapabilityToolSearch) {
		t.Fatal("credential configuration read the capability component")
	}
}

func TestCanonicalLocationCapabilityMovesOnTheNextWrite(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	writePredecessorConfiguration(t, store, toolSearchConfig())
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Routes["claude"].Declares(ProtocolAnthropic, CapabilityToolSearch) {
		t.Fatal("the earlier canonical location was not accepted")
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(before, loaded); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(readStoreFile(t, store.Path()), []byte(CapabilityToolSearch)) {
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

func TestStoredCapabilityAppliesOnlyToItsRecordedRoute(t *testing.T) {
	for _, change := range []string{"none", "upstream model", "endpoint", "account", "interface", "route"} {
		t.Run(change, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
			if err := store.Save(toolSearchConfig()); err != nil {
				t.Fatal(err)
			}
			cfg, err := store.LoadCredentialConfig()
			if err != nil {
				t.Fatal(err)
			}
			route := cfg.Routes["claude"]
			switch change {
			case "upstream model":
				route.UpstreamModel = "claude-other"
			case "endpoint":
				account := cfg.Accounts["ucloud"]
				account.Endpoints.Anthropic = "https://other.test"
				cfg.Accounts["ucloud"] = account
			case "account":
				cfg.Accounts["other"] = cfg.Accounts["ucloud"]
				route.Account = "other"
			case "interface":
				route.Interfaces = map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {CapabilityText}}
			}
			cfg.Routes["claude"] = route
			if change == "route" {
				delete(cfg.Routes, "claude")
			}
			writePredecessorConfiguration(t, store, cfg)
			loaded, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := loaded.Routes["claude"].Declares(ProtocolAnthropic, CapabilityToolSearch), change == "none"; got != want {
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
	responses := capabilityIdentity(fixture, fixture.Routes["gpt"], ProtocolOpenAIResponses)
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
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
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
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := toolSearchConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientClaude}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(readStoreFile(t, store.Path()+".verified.json"), []byte(CapabilityToolSearch)) {
		t.Fatal("verified checkpoint carries an unpublished capability")
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil || !checkpoint.Config.Routes["claude"].Declares(ProtocolAnthropic, CapabilityToolSearch) {
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
	if err != nil || !backup.Routes["claude"].Declares(ProtocolAnthropic, CapabilityToolSearch) {
		t.Fatalf("verified backup lost the stored capability: %v", err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	route := changed.Routes["claude"]
	route.Interfaces = map[EndpointProtocol][]Capability{ProtocolAnthropic: {CapabilityText}}
	changed.Routes["claude"] = route
	after, err := store.Commit(before, changed)
	if err != nil {
		t.Fatal(err)
	}
	if current, err := store.Load(); err != nil || current.Routes["claude"].Declares(ProtocolAnthropic, CapabilityToolSearch) {
		t.Fatalf("committed removal kept the stored capability: %v", err)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Load()
	if err != nil || !restored.Routes["claude"].Declares(ProtocolAnthropic, CapabilityToolSearch) {
		t.Fatalf("compensation did not restore the stored capability: %v", err)
	}
	if got, err := store.CaptureSnapshot(); err != nil || !before.equal(got) {
		t.Fatalf("capability compensation changed the preimage: %v", err)
	}
}

// toolSearchConfig declares every published schema-six capability and the
// later tool_search capability on one Anthropic Route.
func toolSearchConfig() Config {
	cfg := NewConfig()
	cfg.Accounts["ucloud"] = Account{Label: "UCloud", Endpoints: Endpoints{
		Anthropic: "https://ucloud.test", OpenAIResponses: "https://ucloud.test/v1",
	}}
	cfg.Routes["claude"] = Route{
		Label: "Claude", Account: "ucloud", Model: "claude", UpstreamModel: "claude-wire",
		Interfaces: map[EndpointProtocol][]Capability{
			ProtocolAnthropic: append(slices.Clone(publishedSchemaSixCapabilities), CapabilityToolSearch),
		},
	}
	cfg.Routes["gpt"] = Route{
		Label: "GPT", Account: "ucloud", Model: "gpt",
		Interfaces: map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {CapabilityText, CapabilityReasoning}},
	}
	return cfg
}

// writePredecessorConfiguration replaces only the canonical file, as a release
// that knows no capability component writes it.
func writePredecessorConfiguration(t *testing.T, store Store, cfg Config) {
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
