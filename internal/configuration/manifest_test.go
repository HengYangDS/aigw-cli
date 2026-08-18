package configuration

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestManifestAccountNamesReturnsEveryCredentialOwnerOnce(t *testing.T) {
	incoming := Manifest{
		Accounts: map[string]Account{"shared": {}, "direct": {}},
		Profiles: map[string]Profile{"alias": {Account: "shared"}, "implicit": {}},
	}
	want := []string{"direct", "implicit", "shared"}
	if got := ManifestAccountNames(incoming); !reflect.DeepEqual(got, want) {
		t.Fatalf("account names = %#v, want %#v", got, want)
	}
}

func TestCredentialDetectionDescendsIntoArrays(t *testing.T) {
	value := []any{map[string]any{"metadata": map[string]any{"api_token": "secret"}}}
	if got := findCredentialKey(value, "profiles"); got != "profiles.metadata.api_token" {
		t.Fatalf("credential path = %q", got)
	}
}

func TestTeamConfigurationManifestIsReviewedVersionThree(t *testing.T) {
	manifestDirectory := filepath.Join("..", "..", "manifests")
	files, err := filepath.Glob(filepath.Join(manifestDirectory, "*.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "team.toml" {
		t.Fatalf("product manifests = %v, want only the reviewed team manifest", files)
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	parsedManifest, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsedManifest.Version != 3 || len(parsedManifest.Accounts) != 3 || len(parsedManifest.Profiles) == 0 {
		t.Fatalf("team manifest = version %d, %d Accounts, %d profiles", parsedManifest.Version, len(parsedManifest.Accounts), len(parsedManifest.Profiles))
	}
	for accountID, account := range parsedManifest.Accounts {
		for _, endpoint := range []string{account.Endpoints.OpenAIResponses, account.Endpoints.Anthropic} {
			if endpoint == "" {
				continue
			}
			parsed, parseErr := url.Parse(endpoint)
			if parseErr != nil || parsed.Hostname() == "" {
				t.Fatalf("team account %q contains invalid endpoint %q", accountID, endpoint)
			}
		}
	}
	for _, accountID := range []string{"aihubmix", "dmxapi", "ucloud"} {
		if _, ok := parsedManifest.Accounts[accountID]; !ok {
			t.Fatalf("team manifest missing account %q", accountID)
		}
	}
	for _, client := range []string{ClientClaude, ClientCodex} {
		profileID, ok := parsedManifest.RecommendedRoutes[client]
		if !ok {
			t.Fatalf("team manifest missing recommended route for %q", client)
		}
		profile, ok := parsedManifest.Profiles[profileID]
		if !ok || profile.Account != "dmxapi" || profile.Client != client {
			t.Fatalf("recommended %q route = profile %q with invalid ownership: %#v", client, profileID, profile)
		}
		if strings.TrimSpace(profile.Models[client]) == "" {
			t.Fatalf("recommended %q profile %q has no model", client, profileID)
		}
	}
	if parsedManifest.RecommendedDefault != parsedManifest.RecommendedRoutes[ClientCodex] {
		t.Fatalf("team manifest default %q does not match the Codex recommendation %q", parsedManifest.RecommendedDefault, parsedManifest.RecommendedRoutes[ClientCodex])
	}
	if len(parsedManifest.RecommendedRoutes) != 2 {
		t.Fatalf("team manifest recommended routes = %#v", parsedManifest.RecommendedRoutes)
	}
}

func TestRecommendedRoutesAreValidatedExportedAndDoNotOverridePersonalChoices(t *testing.T) {
	raw := []byte(`version = 3
recommended_default = "team-codex"
[recommended_routes]
claude = "team-claude"
codex = "team-codex"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
anthropic = "https://team.test"

[profiles.team-claude]
label = "Team Claude"
account = "team"
client = "claude"
[profiles.team-claude.models]
claude = "claude-test"

[profiles.team-codex]
label = "Team Codex"
account = "team"
client = "codex"
[profiles.team-codex.models]
codex = "gpt-test"
`)
	team, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Merge(NewConfig(), team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Overrides[ClientClaude] != "team-claude" || got.Routes.Overrides[ClientCodex] != "team-codex" {
		t.Fatalf("recommended routes not applied: %#v", got.Routes)
	}

	got.Accounts["personal"] = Account{Label: "Personal", Endpoints: Endpoints{Anthropic: "https://personal.test"}}
	got.Profiles["personal-claude"] = Profile{Label: "Personal Claude", Account: "personal", Client: ClientClaude, Models: Models{ClientClaude: "personal-model"}}
	got.Routes.Overrides[ClientClaude] = "personal-claude"
	merged, err := Merge(got, team)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Routes.Overrides[ClientClaude] != "personal-claude" || merged.Routes.Overrides[ClientCodex] != "team-codex" {
		t.Fatalf("merge replaced a personal route: %#v", merged.Routes)
	}

	exported, err := Export(merged)
	if err != nil {
		t.Fatal(err)
	}
	exportedTeam, err := Parse(exported)
	if err != nil {
		t.Fatal(err)
	}
	if exportedTeam.RecommendedRoutes[ClientClaude] != "personal-claude" || exportedTeam.RecommendedRoutes[ClientCodex] != "team-codex" {
		t.Fatalf("exported routes = %#v", exportedTeam.RecommendedRoutes)
	}
}

func TestManifestAdmissionIsDerivedFromClientRegistry(t *testing.T) {
	previous := admittedClientSpecs
	admittedClientSpecs = append(AdmittedClientSpecs(), ClientSpec{
		ID:               "synthetic",
		Label:            "Synthetic",
		EndpointProtocol: ProtocolOpenAIResponses,
	})
	defer func() { admittedClientSpecs = previous }()

	manifest, err := Parse([]byte(`version = 3
recommended_default = "synthetic-default"
[recommended_routes]
synthetic = "synthetic-default"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"

[profiles.synthetic-default]
label = "Synthetic Default"
account = "team"
client = "synthetic"
[profiles.synthetic-default.models]
synthetic = "synthetic-model"
`))
	if err != nil {
		t.Fatalf("registry-admitted client was rejected: %v", err)
	}
	if manifest.RecommendedRoutes["synthetic"] != "synthetic-default" {
		t.Fatalf("recommended routes = %#v", manifest.RecommendedRoutes)
	}
}

func TestParseRejectsIncompatibleRecommendedRoute(t *testing.T) {
	raw := []byte(`version = 3
recommended_default = "codex"
[recommended_routes]
claude = "codex"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
[profiles.codex]
label = "Codex"
account = "team"
client = "codex"
[profiles.codex.models]
codex = "gpt-test"
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("incompatible recommended route was accepted")
	}
}

func TestVersionTwoIsRejected(t *testing.T) {
	legacy := []byte(`version = 2
recommended_default = "claude"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.claude]
label = "Claude"
account = "team"
client = "claude"
[profiles.claude.models]
claude = "claude-test"
`)
	if _, err := Parse(legacy); err == nil || !strings.Contains(err.Error(), "expected 3") {
		t.Fatalf("v2 manifest error = %v", err)
	}
}

func TestExportRejectsRouteThatCannotBeParsedBack(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "team", Client: ClientCodex, Models: Models{ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "codex"
	cfg.Routes.Overrides[ClientClaude] = "codex"

	if _, err := Export(cfg); err == nil {
		t.Fatal("export accepted a client-incompatible route")
	}
}

func TestParseConfigurationManifestAndMergePreservesPersonalState(t *testing.T) {
	raw := []byte(`version = 3
recommended_default = "team"

[accounts.team]
label = "Team Gateway"

[accounts.team.endpoints]
openai_responses = "https://gateway.test/v1"
anthropic = "https://gateway.test"

[profiles.team]
label = "Team Gateway"
account = "team"
`)
	m, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Adapters[ClientClaude] = AdapterConfig{Enabled: true, Executable: "/personal/claude"}
	cfg.Routes.Overrides[ClientClaude] = "personal"
	cfg.Accounts["personal"] = Account{Label: "Personal", Endpoints: Endpoints{Anthropic: "https://personal.test"}}
	cfg.Profiles["personal"] = Profile{Label: "Personal", Account: "personal"}
	cfg.Routes.Default = "personal"
	got, err := Merge(cfg, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "personal" || got.Routes.Overrides[ClientClaude] != "personal" {
		t.Fatalf("personal routes changed: %#v", got.Routes)
	}
	if got.Adapters[ClientClaude].Executable != "/personal/claude" {
		t.Fatalf("personal adapter changed: %#v", got.Adapters)
	}
	if got.Profiles["team"].Label != "Team Gateway" {
		t.Fatalf("imported profile missing: %#v", got.Profiles)
	}
}

func TestMergeRejectsConflictingExistingAccountWithoutMutatingLocalConfig(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "team-profile"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team-profile]
label = "TeamProfile"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Personal Gateway", Endpoints: Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "team"}
	cfg.Routes.Default = "local"

	_, err = Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `account "team" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if !strings.Contains(err.Error(), "aigw config export") || strings.Contains(err.Error(), "aigw account list") {
		t.Fatalf("merge guidance must name an existing inspection command: %v", err)
	}
	if got := cfg.Accounts["team"].Endpoints.Anthropic; got != "https://personal.example.test" {
		t.Fatalf("conflicting merge mutated existing endpoint: %q", got)
	}
	if _, exists := cfg.Profiles["team-profile"]; exists {
		t.Fatalf("conflicting merge partially imported profile: %#v", cfg.Profiles)
	}
}

func TestMergeRejectsConflictingExistingProfileWithoutMutatingLocalConfig(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "shared"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[profiles.shared]
label = "Team Model"
account = "team"
client = "codex"
[profiles.shared.models]
codex = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Profiles["shared"] = Profile{Label: "Personal Model", Account: "team", Client: ClientCodex, Models: Models{ClientCodex: "personal-model"}}
	cfg.Routes.Default = "shared"

	_, err = Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `profile "shared" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if got := cfg.Profiles["shared"].Models[ClientCodex]; got != "personal-model" {
		t.Fatalf("conflicting merge mutated active profile model: %q", got)
	}
}

func TestMergeAcceptsEquivalentExistingIdentityWithoutReplacingLocalState(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "shared"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.shared]
label = "TeamProfile"
purpose = "Default agent"
account = "team"
client = "claude"
[profiles.shared.models]
claude = "claude-team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{Anthropic: "https://team.example.test/"}}
	cfg.Profiles["shared"] = Profile{Label: "TeamProfile", Purpose: "Default agent", Account: "team", Client: ClientClaude, Models: Models{ClientClaude: "claude-team"}}
	cfg.Routes.Default = "shared"

	got, err := Merge(cfg, team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "shared" || got.Profiles["shared"].Models[ClientClaude] != "claude-team" {
		t.Fatalf("equivalent merge = %#v", got)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://team.example.test/" {
		t.Fatalf("idempotent import should preserve local canonical representation: %#v", got.Accounts["team"])
	}
}

func TestMergeWithOptionsReplacesOnlyExplicitConflictingIdentity(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "shared"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[profiles.shared]
label = "Team Model"
account = "team"
client = "codex"
[profiles.shared.models]
codex = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Personal Gateway", Endpoints: Endpoints{OpenAIResponses: "https://personal.example.test/v1"}}
	cfg.Profiles["shared"] = Profile{Label: "Personal Model", Account: "team", Client: ClientCodex, Models: Models{ClientCodex: "personal-model"}}
	cfg.Routes.Default = "shared"

	got, err := MergeWithOptions(cfg, team, MergeOptions{
		ReplaceAccounts: map[string]bool{"team": true},
		ReplaceProfiles: map[string]bool{"shared": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.OpenAIResponses != "https://team.example.test/v1" {
		t.Fatalf("account replacement = %#v", got.Accounts["team"])
	}
	if got.Profiles["shared"].Models[ClientCodex] != "team-model" {
		t.Fatalf("profile replacement = %#v", got.Profiles["shared"])
	}
}

func TestMergeWithOptionsRejectsUnusedReplacementSelectors(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "team-profile"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team-profile]
label = "TeamProfile"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["local"] = Account{Label: "Local", Endpoints: Endpoints{Anthropic: "https://local.example.test"}}
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "local"}
	cfg.Routes.Default = "local"

	_, err = MergeWithOptions(cfg, team, MergeOptions{ReplaceAccounts: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-account "missing"`) {
		t.Fatalf("unused account replacement error = %v", err)
	}
	_, err = MergeWithOptions(cfg, team, MergeOptions{ReplaceProfiles: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-profile "missing"`) {
		t.Fatalf("unused profile replacement error = %v", err)
	}
}

func TestMergeWithOptionsDoesNotNormalizeOrMutateRejectedInput(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "team"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team]
label = "Team"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Version: ConfigVersion,
		Accounts: map[string]Account{
			"team": {Label: "Personal Gateway", Endpoints: Endpoints{Anthropic: "https://personal.example.test"}},
		},
		Profiles: map[string]Profile{
			"local": {Label: "Local", Account: "team"},
		},
		Routes: Routes{Default: "local"},
	}

	_, err = MergeWithOptions(cfg, team, MergeOptions{})
	if err == nil {
		t.Fatal("expected conflict")
	}
	if cfg.Routes.Overrides != nil || cfg.Adapters != nil {
		t.Fatalf("rejected merge normalized caller-owned config: %#v", cfg)
	}
}

func TestParseRejectsCredentialShapedFields(t *testing.T) {
	for _, key := range []string{"token", "api_key", "password", "auth_header", "client_secret"} {
		raw := []byte("version = 3\nrecommended_default = \"team\"\n" + key + " = \"must-not-exist\"\n")
		_, err := Parse(raw)
		if err == nil || !strings.Contains(err.Error(), "credential") {
			t.Errorf("key %s: error = %v", key, err)
		}
	}
}

func TestParseRejectsNonCanonicalSchemaVersion(t *testing.T) {
	oldSchema := []byte(`version = 1
recommended_default = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[profiles.team]
label = "Team"
purpose = "Default agent"
account = "team"
`)
	if _, err := Parse(oldSchema); err == nil || !strings.Contains(err.Error(), "unsupported configuration manifest version 1") {
		t.Fatalf("version 1 parse error = %v", err)
	}
	current := []byte(strings.Replace(string(oldSchema), "version = 1", "version = 3", 1))
	parsed, err := Parse(current)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version != 3 || parsed.Profiles["team"].Purpose != "Default agent" {
		t.Fatalf("parsed manifest = %#v", parsed)
	}
}

func TestMergeRejectsNonCanonicalLocalSchemaVersion(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[profiles.team]
label = "Team"
purpose = "Default agent"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Version = 1
	cfg.Accounts["local"] = Account{Label: "Local", Endpoints: Endpoints{Anthropic: "https://local.test"}}
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "local"}
	cfg.Routes.Default = "local"
	if _, err := Merge(cfg, team); err == nil || !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("merge error = %v", err)
	}
}

func TestParseRejectsProfileOwnedEndpointResidue(t *testing.T) {
	raw := []byte(`version = 3
recommended_default = "team"

[profiles.team]
label = "Team Gateway"
account = "team"

[profiles.team.endpoints]
openai_responses = "https://gateway.test/v1"
`)
	if _, err := Parse(raw); err == nil {
		t.Fatalf("legacyProfile endpoint error = %v", err)
	}
}

func TestParseRejectsMalformedTOML(t *testing.T) {
	if _, err := Parse([]byte("version = [1, 2\n")); err == nil || !strings.Contains(err.Error(), "parse configuration manifest") {
		t.Fatalf("malformed TOML error = %v", err)
	}
}

func TestParseRejectsManifestWithoutAnyProfile(t *testing.T) {
	raw := []byte(`version = 3
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "at least one profile") {
		t.Fatalf("no-profile error = %v", err)
	}
}

func TestParseRejectsUnknownRecommendedDefault(t *testing.T) {
	raw := []byte(`version = 3
recommended_default = "missing"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "recommended default references unknown profile") {
		t.Fatalf("unknown recommended default error = %v", err)
	}
}

func TestParseInitializesMissingAccountsAndDefaultsToFirstProfile(t *testing.T) {
	raw := []byte(`version = 3
[profiles.solo]
label = "Solo"
account = "missing"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "invalid configuration manifest") {
		t.Fatalf("account-less manifest error = %v", err)
	}
}

func TestParseRejectsRecommendedRouteWithUnsupportedClient(t *testing.T) {
	raw := []byte(`version = 3
[recommended_routes]
gemini = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "unsupported client") {
		t.Fatalf("unsupported recommended route client error = %v", err)
	}
}

func TestParseRejectsRecommendedRouteReferencingUnknownProfile(t *testing.T) {
	raw := []byte(`version = 3
[recommended_routes]
claude = "missing"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "references unknown profile") {
		t.Fatalf("unknown recommended route profile error = %v", err)
	}
}

func TestParseRejectsDeeplyNestedCredentialShapedFields(t *testing.T) {
	raw := []byte(`version = 3
[wrapper]
password = "leak"
[[entries]]
token = "leak"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("nested credential field error = %v", err)
	}
}

func TestMergeWithOptionsRejectsNonCanonicalConfigurationManifestVersion(t *testing.T) {
	team := Manifest{Version: 99, Profiles: map[string]Profile{"team": {Label: "Team", Account: "team"}}}
	if _, err := MergeWithOptions(NewConfig(), team, MergeOptions{}); err == nil || !strings.Contains(err.Error(), "unsupported configuration manifest version 99") {
		t.Fatalf("unsupported manifest version error = %v", err)
	}
}

func TestMergeDefaultsToFirstImportedProfileWhenNeitherSideChoosesADefault(t *testing.T) {
	team, err := Parse([]byte(`version = 3
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.solo]
label = "Solo"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Merge(NewConfig(), team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "solo" {
		t.Fatalf("merged default = %q, want the only imported profile", got.Routes.Default)
	}
}

func TestMergeRejectsConflictingModelOverrideWithOtherwiseIdenticalProfile(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "shared"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"
[profiles.shared]
label = "TeamProfile"
account = "team"
client = "codex"
[profiles.shared.models]
codex = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Profiles["shared"] = Profile{Label: "TeamProfile", Account: "team", Client: ClientCodex, Models: Models{ClientCodex: "personal-model"}}
	cfg.Routes.Default = "shared"

	if _, err := Merge(cfg, team); err == nil || !strings.Contains(err.Error(), `profile "shared" conflicts`) {
		t.Fatalf("model-only conflict error = %v", err)
	}
}

func TestMergeTreatsIdenticalAccountProbesAsEquivalent(t *testing.T) {
	team, err := Parse([]byte(`version = 3
recommended_default = "team"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"
[accounts.team.account_probe]
kind = "dmxapi"
base_url = "https://team.example.test/probe"
[profiles.team]
label = "Team"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{
		Label:        "Team Gateway",
		Endpoints:    Endpoints{Anthropic: "https://team.example.test"},
		AccountProbe: &AccountProbe{Kind: "dmxapi", BaseURL: "https://team.example.test/probe/"},
	}
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "team"}
	cfg.Routes.Default = "local"

	got, err := Merge(cfg, team)
	if err != nil {
		t.Fatalf("equivalent account probes must not conflict: %v", err)
	}
	if got.Accounts["team"].AccountProbe.Kind != "dmxapi" {
		t.Fatalf("merged account probe = %#v", got.Accounts["team"].AccountProbe)
	}
}

func TestExportRejectsInvalidConfiguration(t *testing.T) {
	if _, err := Export(Config{}); err == nil {
		t.Fatal("export accepted an invalid configuration")
	}
}

func TestExportOmitsSecretsAndAdaptersAndPublishesRouteRecommendations(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Profiles["team"] = Profile{Label: "Team", Account: "team"}
	cfg.Routes.Default = "team"
	cfg.Routes.Overrides[ClientClaude] = "team"
	cfg.Adapters[ClientClaude] = AdapterConfig{Enabled: true, Executable: "/personal/claude"}
	data, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"adapters", "overrides", "/personal/claude", "token", "secret"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("export contains %q:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, `recommended_default = 'team'`) && !strings.Contains(text, `recommended_default = "team"`) {
		t.Fatalf("export lacks recommended default:\n%s", text)
	}
	if !strings.Contains(text, "version = 3") || !strings.Contains(text, "recommended_routes") || !strings.Contains(text, "claude") {
		t.Fatalf("new config export must use manifest v3 with route recommendations:\n%s", text)
	}
}
