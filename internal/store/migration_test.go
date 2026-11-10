package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

func publishedV010Store(t *testing.T) (Store, []byte, string, string, string) {
	t.Helper()
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "config.toml"), testAdmission())
	executable := filepath.Join(root, "bin", "codex")
	target := filepath.Join(root, "home", ".codex", "config.toml")
	credentialCommand := filepath.Join(root, "bin", "aigw")
	predecessor := []byte(fmt.Sprintf(`version = 3

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
anthropic = "https://gateway.test"
openai_responses = "https://gateway.test/v1"

[profiles.claude]
label = "Claude"
account = "gateway"
client = "claude"
model = "claude-test"

[profiles.codex]
label = "Codex"
account = "gateway"
client = "codex"
model = "gpt-test"
model_provider = "amazon-bedrock"
authentication = "client-native"

[routes]
claude = "claude"
codex = "codex"

[recommended_routes]
codex = "codex"

[adapters.claude]
enabled = false

[adapters.codex]
enabled = true
executable = %q
targets = [%q]
credential_command = %q
`, executable, target, credentialCommand))
	if err := os.WriteFile(store.Path(), predecessor, 0o600); err != nil {
		t.Fatal(err)
	}
	return store, predecessor, executable, target, credentialCommand
}

func TestPrepareMigrationPreservesPublishedV010Selection(t *testing.T) {
	store, predecessor, executable, target, credentialCommand := publishedV010Store(t)
	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Required || plan.FromVersion != 3 || plan.ToVersion != domain.ConfigVersion {
		t.Fatalf("published predecessor migration = %#v", plan)
	}
	if actual := plan.Clients[ClientClaude]; !reflect.DeepEqual(actual, domain.ClientBinding{Model: "claude", Protocol: domain.ProtocolAnthropic}) {
		t.Fatalf("disabled Claude selection = %#v", actual)
	}
	wantCodex := domain.ClientBinding{
		Model: "codex", Enabled: true, Protocol: domain.ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative,
		Executable: executable, Targets: []string{target}, CredentialCommand: credentialCommand,
	}
	if actual := plan.Clients[ClientCodex]; !reflect.DeepEqual(actual, wantCodex) {
		t.Fatalf("Codex selection = %#v", actual)
	}
	wantRecommendation := domain.ClientRecommendation{Primary: domain.ClientSelection{
		Model: "codex", Protocol: domain.ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative,
	}}
	if actual := plan.Recommendations[ClientCodex]; !reflect.DeepEqual(actual, wantRecommendation) {
		t.Fatalf("Codex recommendation = %#v", actual)
	}
	if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, predecessor) {
		t.Fatalf("migration preview changed published predecessor: %v", err)
	}
}

func appliedPublishedV010Store(t *testing.T) (Store, domain.Config, []byte) {
	t.Helper()
	store, predecessor, _, _, _ := publishedV010Store(t)
	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyMigration(plan); err != nil {
		t.Fatal(err)
	}
	migrated, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated.Families) != 2 || len(migrated.Models) != 2 {
		t.Fatalf("published predecessor identities were lost: models=%d routes=%d", len(migrated.Families), len(migrated.Models))
	}
	for id, route := range migrated.Models {
		if route.UpstreamModel != route.Family || len(route.Protocols) != 1 {
			t.Fatalf("route %q changed upstream identity or invented protocols: %#v", id, route)
		}
		for _, capabilities := range route.Protocols {
			if len(capabilities) != 0 {
				t.Fatalf("route %q invented capabilities: %v", id, capabilities)
			}
		}
	}
	return store, migrated, predecessor
}

func TestPublishedV010NoOpPreservesFormattingAndRollbackInput(t *testing.T) {
	store, migrated, predecessor := appliedPublishedV010Store(t)
	beforeNoOp, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(migrated); err != nil {
		t.Fatal(err)
	}
	afterNoOp, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !beforeNoOp.Config.Equal(afterNoOp.Config) || !beforeNoOp.Backup.Equal(afterNoOp.Backup) || !beforeNoOp.Verified.Equal(afterNoOp.Verified) {
		t.Fatal("an unchanged migrated configuration rewrote its rollback input")
	}
	annotated := append([]byte("# operator-owned formatting\n"), afterNoOp.Config.Data...)
	if err := os.WriteFile(store.Path(), annotated, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(migrated); err != nil {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, annotated) {
		t.Fatalf("an unchanged migrated configuration lost operator-owned formatting: %v", err)
	}
	if backup, err := os.ReadFile(store.Path() + ".bak"); err != nil || !bytes.Equal(backup, predecessor) {
		t.Fatalf("an unchanged migrated configuration lost its predecessor: %v", err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), migrated, sessions(ClientCodex)); err != nil {
		t.Fatal(err)
	}
	verified, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConvergeVerifiedBackup(verified.Snapshot); err != nil {
		t.Fatal(err)
	}
	if backup, err := os.ReadFile(store.Path() + ".bak"); err != nil || !bytes.Equal(backup, predecessor) {
		t.Fatalf("published predecessor rollback input changed: %v", err)
	}
}

func TestPublishedV010RollbackRestoresExactBytes(t *testing.T) {
	store, _, predecessor := appliedPublishedV010Store(t)
	rollback, err := store.PrepareMigration(true)
	if err != nil {
		t.Fatal(err)
	}
	if rollback.FromVersion != domain.ConfigVersion || rollback.ToVersion != 3 {
		t.Fatalf("published predecessor rollback = %#v", rollback)
	}
	if err := store.ApplyMigration(rollback); err != nil {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, predecessor) {
		t.Fatalf("published predecessor was not restored byte for byte: %v", err)
	}
}

// A predecessor schema is the matching release's to read: no command of this
// program migrates it.
func TestPublishedConfigurationVersionDiagnosticNamesTheMatchingRelease(t *testing.T) {
	message := (&domain.UnsupportedConfigVersionError{Version: domain.PublishedConfigVersion, ExpectedVersion: domain.ConfigVersion}).Error()
	if !strings.Contains(message, "use the matching AIGW release") || strings.Contains(message, "aigw config") {
		t.Fatalf("published predecessor diagnostic = %s", message)
	}
}

func TestPrepareMigrationTranslatesImmediatePredecessorWithoutInventingCapabilities(t *testing.T) {
	store, original := legacyStore(t)

	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.FromVersion != domain.LegacyConfigVersion || plan.ToVersion != domain.ConfigVersion ||
		len(plan.Providers) != 1 || len(plan.Models) != 2 ||
		len(plan.Clients) != 2 || len(plan.Recommendations) != 1 {
		t.Fatalf("migration plan = %#v", plan)
	}
	if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, original) {
		t.Fatalf("preview changed configuration: %q, %v", actual, err)
	}

	root := filepath.Dir(store.Path())
	wantClaude := domain.ClientBinding{Model: "claude", Protocol: domain.ProtocolAnthropic, Executable: filepath.Join(root, "bin", "claude")}
	if actual := plan.Clients[ClientClaude]; !reflect.DeepEqual(actual, wantClaude) {
		t.Fatalf("Claude binding = %#v", actual)
	}
	wantCodex := domain.ClientBinding{
		Model: "codex", Enabled: true, Protocol: domain.ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative,
		Executable: filepath.Join(root, "bin", "codex"), Targets: []string{filepath.Join(root, "home", ".codex", "config.toml")},
		CredentialCommand: filepath.Join(root, "bin", "aigw"),
	}
	if actual := plan.Clients[ClientCodex]; !reflect.DeepEqual(actual, wantCodex) {
		t.Fatalf("Codex binding = %#v", actual)
	}
	wantRecommendation := domain.ClientRecommendation{Primary: domain.ClientSelection{
		Model: "codex", Protocol: domain.ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative,
	}}
	if actual := plan.Recommendations[ClientCodex]; !reflect.DeepEqual(actual, wantRecommendation) {
		t.Fatalf("Codex recommendation = %#v", actual)
	}

	legacy, err := decodeLegacyConfig(original)
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := migrateLegacyConfig(legacy, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	for id, route := range migrated.Models {
		if route.UpstreamModel != route.Family {
			t.Fatalf("route %q upstream model = %q, want exact predecessor model %q", id, route.UpstreamModel, route.Family)
		}
		for protocol, capabilities := range route.Protocols {
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
	if rollback.FromVersion != domain.ConfigVersion || rollback.ToVersion != domain.LegacyConfigVersion {
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
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Models["codex"] = domain.Model{
		Label: "Codex", Provider: "gateway", Family: "gpt-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedModel(ClientCodex, "codex", "")
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
	if plan.Required || plan.FromVersion != domain.ConfigVersion || plan.ToVersion != domain.ConfigVersion {
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

func TestPrepareMigrationRejectsUnqualifiedPredecessorModel(t *testing.T) {
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
		Version: domain.LegacyConfigVersion,
		Providers: map[string]domain.Provider{"gateway": {
			Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"},
		}},
		Profiles: map[string]legacyProfile{
			ClientClaude: {Label: "Claude", Purpose: "Conversation", Provider: "gateway", Model: "claude-test", Protocols: []domain.EndpointProtocol{domain.ProtocolAnthropic}},
			ClientCodex:  {Label: "Codex", Provider: "gateway", Model: "gpt-test", Tier: "flagship", Protocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses}},
		},
		Recommendations: map[string]legacySelection{ClientCodex: {
			Profile: ClientCodex, Protocol: domain.ProtocolOpenAIResponses,
			ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative,
		}},
		Clients: map[string]legacyBinding{
			ClientClaude: {Profile: ClientClaude, Protocol: domain.ProtocolAnthropic, Executable: filepath.Join(root, "bin", "claude")},
			ClientCodex: {
				Profile: ClientCodex, Enabled: true, Protocol: domain.ProtocolOpenAIResponses,
				ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative,
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
	return NewStore(path, testAdmission()), data
}
