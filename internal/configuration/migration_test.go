package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestPrepareMigrationTranslatesImmediatePredecessorWithoutInventingCapabilities(t *testing.T) {
	store, original := legacyStore(t)

	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.FromVersion != LegacyConfigVersion || plan.ToVersion != ConfigVersion ||
		len(plan.Accounts) != 1 || len(plan.Profiles) != 2 ||
		len(plan.Clients) != 2 || len(plan.Recommendations) != 1 {
		t.Fatalf("migration plan = %#v", plan)
	}
	if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, original) {
		t.Fatalf("preview changed configuration: %q, %v", actual, err)
	}

	root := filepath.Dir(store.Path())
	wantClaude := ClientBinding{Route: "claude", Protocol: ProtocolAnthropic, Executable: filepath.Join(root, "bin", "claude")}
	if actual := plan.Clients[ClientClaude]; !reflect.DeepEqual(actual, wantClaude) {
		t.Fatalf("Claude binding = %#v", actual)
	}
	wantCodex := ClientBinding{
		Route: "codex", Enabled: true, Protocol: ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative,
		Executable: filepath.Join(root, "bin", "codex"), Targets: []string{filepath.Join(root, "home", ".codex", "config.toml")},
		CredentialCommand: filepath.Join(root, "bin", "aigw"),
	}
	if actual := plan.Clients[ClientCodex]; !reflect.DeepEqual(actual, wantCodex) {
		t.Fatalf("Codex binding = %#v", actual)
	}
	wantRecommendation := ClientRecommendation{Primary: ClientSelection{
		Route: "codex", Protocol: ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative,
	}}
	if actual := plan.Recommendations[ClientCodex]; !reflect.DeepEqual(actual, wantRecommendation) {
		t.Fatalf("Codex recommendation = %#v", actual)
	}

	legacy, err := decodeLegacyConfig(original)
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := migrateLegacyConfig(legacy)
	if err != nil {
		t.Fatal(err)
	}
	for id, route := range migrated.Routes {
		if route.UpstreamModel != route.Model {
			t.Fatalf("route %q upstream model = %q, want exact predecessor model %q", id, route.UpstreamModel, route.Model)
		}
		for protocol, capabilities := range route.Interfaces {
			if len(capabilities) != 0 {
				t.Fatalf("route %q interface %q invented capabilities: %v", id, protocol, capabilities)
			}
		}
	}
}

func TestApplyMigrationRetainsExactPredecessorBytesAndRollbackSwapsThemBack(t *testing.T) {
	store, predecessor := legacyStore(t)
	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyMigration(plan); err != nil {
		t.Fatal(err)
	}
	migrated, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(migrated, []byte("[profiles]")) || bytes.Contains(migrated, []byte("tier =")) ||
		bytes.Contains(migrated, []byte("protocols =")) || !bytes.Contains(migrated, []byte("[clients.codex]")) {
		t.Fatalf("migration retained predecessor authority:\n%s", migrated)
	}
	if backup, err := os.ReadFile(store.Path() + ".bak"); err != nil || !bytes.Equal(backup, predecessor) {
		t.Fatalf("predecessor rollback input = %q, %v", backup, err)
	}

	rollback, err := store.PrepareMigration(true)
	if err != nil {
		t.Fatal(err)
	}
	if rollback.FromVersion != ConfigVersion || rollback.ToVersion != LegacyConfigVersion {
		t.Fatalf("rollback plan = %#v", rollback)
	}
	if err := store.ApplyMigration(rollback); err != nil {
		t.Fatal(err)
	}
	if restored, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(restored, predecessor) {
		t.Fatalf("restored predecessor configuration = %q, %v", restored, err)
	}
	if backup, err := os.ReadFile(store.Path() + ".bak"); err != nil || !bytes.Equal(backup, migrated) {
		t.Fatalf("forward-recovery input = %q, %v", backup, err)
	}
}

func TestApplyMigrationRejectsChangedPreimage(t *testing.T) {
	store, _ := legacyStore(t)
	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	newer := []byte("version = 5\n# newer operator state\n")
	if err := os.WriteFile(store.Path(), newer, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyMigration(plan); err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("changed preimage error = %v", err)
	}
	if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, newer) {
		t.Fatalf("changed preimage was overwritten: %q, %v", actual, err)
	}
}

func TestCurrentConfigurationMigrationIsANoOp(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := NewConfig()
	cfg.Accounts["gateway"] = Account{Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["codex"] = Route{
		Label: "Codex", Account: "gateway", Model: "gpt-test",
		Interfaces: map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(ClientCodex, "codex")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Required || plan.FromVersion != ConfigVersion || plan.ToVersion != ConfigVersion {
		t.Fatalf("current migration plan = %#v", plan)
	}
	if err := store.ApplyMigration(plan); err != nil {
		t.Fatal(err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("no-op migration changed state: before=%#v after=%#v", before, after)
	}
}

func TestPrepareMigrationRejectsUnqualifiedPredecessorRoute(t *testing.T) {
	store, _ := legacyStoreWith(t, func(config *legacyConfig) {
		profile := config.Profiles[ClientCodex]
		profile.Protocols = nil
		config.Profiles[ClientCodex] = profile
	})
	if _, err := store.PrepareMigration(false); err == nil || !strings.Contains(err.Error(), "cannot be migrated without guessing") {
		t.Fatalf("unqualified predecessor error = %v", err)
	}
}

func legacyStore(t *testing.T) (Store, []byte) {
	return legacyStoreWith(t, nil)
}

func legacyStoreWith(t *testing.T, mutate func(*legacyConfig)) (Store, []byte) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	config := legacyConfig{
		Version: LegacyConfigVersion,
		Accounts: map[string]Account{"gateway": {
			Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"},
		}},
		Profiles: map[string]legacyProfile{
			ClientClaude: {Label: "Claude", Purpose: "Conversation", Account: "gateway", Model: "claude-test", Protocols: []EndpointProtocol{ProtocolAnthropic}},
			ClientCodex:  {Label: "Codex", Account: "gateway", Model: "gpt-test", Tier: "flagship", Protocols: []EndpointProtocol{ProtocolOpenAIResponses}},
		},
		Recommendations: map[string]legacySelection{ClientCodex: {
			Profile: ClientCodex, Protocol: ProtocolOpenAIResponses,
			ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative,
		}},
		Clients: map[string]legacyBinding{
			ClientClaude: {Profile: ClientClaude, Protocol: ProtocolAnthropic, Executable: filepath.Join(root, "bin", "claude")},
			ClientCodex: {
				Profile: ClientCodex, Enabled: true, Protocol: ProtocolOpenAIResponses,
				ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative,
				Executable: filepath.Join(root, "bin", "codex"), Targets: []string{filepath.Join(root, "home", ".codex", "config.toml")},
				CredentialCommand: filepath.Join(root, "bin", "aigw"),
			},
		},
	}
	if mutate != nil {
		mutate(&config)
	}
	data, err := toml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return NewStore(path), data
}
