package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const legacyConfiguration = `version = 3

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
openai_responses = "https://gateway.test/v1"
anthropic = "https://gateway.test"

[profiles.claude]
label = "Claude"
purpose = "Conversation"
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
executable = "/opt/claude"

[adapters.codex]
enabled = true
executable = "/opt/codex"
targets = ["/home/member/.codex/config.toml"]
credential_command = "/opt/aigw"
`

func TestPrepareMigrationTranslatesLegacySelectionOwnership(t *testing.T) {
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

	claude := plan.Clients[ClientClaude]
	wantClaude := ClientBinding{Profile: "claude", Protocol: ProtocolAnthropic, Executable: "/opt/claude"}
	if !reflect.DeepEqual(claude, wantClaude) {
		t.Fatalf("Claude binding = %#v", claude)
	}
	codex := plan.Clients[ClientCodex]
	wantCodex := ClientBinding{
		Profile: "codex", Enabled: true, Protocol: ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative,
		Executable: "/opt/codex", Targets: []string{"/home/member/.codex/config.toml"}, CredentialCommand: "/opt/aigw",
	}
	if !reflect.DeepEqual(codex, wantCodex) {
		t.Fatalf("Codex binding = %#v", codex)
	}
	wantRecommendation := ClientSelection{
		Profile: "codex", Protocol: ProtocolOpenAIResponses,
		ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative,
	}
	if recommendation := plan.Recommendations[ClientCodex]; !reflect.DeepEqual(recommendation, wantRecommendation) {
		t.Fatalf("Codex recommendation = %#v", recommendation)
	}
}

func TestPrepareMigrationPreservesExplicitDisabledIntentWithoutASelection(t *testing.T) {
	store, _ := legacyStore(t)
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("[adapters.claude]\nenabled = false\nexecutable = \"/opt/claude\"\n\n"), []byte("[adapters.claude]\nenabled = false\n\n"), 1)
	data = bytes.Replace(data, []byte("claude = \"claude\"\n"), nil, 1)
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}

	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	if binding, exists := plan.Clients[ClientClaude]; !exists || !reflect.DeepEqual(binding, ClientBinding{}) {
		t.Fatalf("disabled intent = %#v, exists = %t", binding, exists)
	}
}

func TestApplyMigrationRetainsExactLegacyBytesAndRollbackSwapsThemBack(t *testing.T) {
	store, legacy := legacyStore(t)
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
	if bytes.Contains(migrated, []byte("[routes]")) || bytes.Contains(migrated, []byte("[adapters")) ||
		!bytes.Contains(migrated, []byte("[clients.codex]")) {
		t.Fatalf("migration retained parallel selection state:\n%s", migrated)
	}
	if backup, err := os.ReadFile(store.Path() + ".bak"); err != nil || !bytes.Equal(backup, legacy) {
		t.Fatalf("legacy rollback input = %q, %v", backup, err)
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
	if restored, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(restored, legacy) {
		t.Fatalf("restored legacy configuration = %q, %v", restored, err)
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
	newer := []byte("version = 3\n# newer operator state\n")
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
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "gateway", Model: "gpt-test"}
	cfg.SetSelectedProfile(ClientCodex, "codex")
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

func TestPrepareMigrationRejectsAmbiguousLegacyClientOptions(t *testing.T) {
	store, _ := legacyStore(t)
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("codex = \"codex\"\n\n[recommended_routes]"), []byte("\n[recommended_routes]"), 1)
	data = bytes.Replace(data, []byte("codex = \"codex\"\n\n[adapters.claude]"), []byte("\n[adapters.claude]"), 1)
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PrepareMigration(false); err == nil || !strings.Contains(err.Error(), "client-specific options") {
		t.Fatalf("ambiguous migration error = %v", err)
	}
}

func TestPrepareMigrationRejectsInvalidLegacyProfileClient(t *testing.T) {
	store, _ := legacyStore(t)
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("client = \"claude\""), []byte("client = \"unknown\""), 1)
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PrepareMigration(false); err == nil || !strings.Contains(err.Error(), "unknown client") {
		t.Fatalf("invalid legacy client error = %v", err)
	}
}

func legacyStore(t *testing.T) (Store, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(legacyConfiguration)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return NewStore(path), data
}
