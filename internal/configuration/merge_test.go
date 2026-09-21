package configuration

import (
	"strings"
	"testing"
)

func TestRecommendationsAreValidatedExportedAndDoNotOverridePersonalChoices(t *testing.T) {
	raw := []byte(`version = 6
[recommendations.claude]
profile = "team-claude"

[recommendations.codex]
profile = "team-codex"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
anthropic = "https://team.test"

[profiles.team-claude]
label = "Team Claude"
account = "team"
model = "claude-test"

[profiles.team-codex]
label = "Team Codex"
account = "team"
model = "gpt-test"
`)
	team, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Merge(NewConfig(), team)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("import converted recommendations into client bindings: %#v", got.Clients)
	}

	got.Accounts["personal"] = Account{Label: "Personal", Endpoints: Endpoints{Anthropic: "https://personal.test"}}
	got.Profiles["personal-claude"] = Profile{Label: "Personal Claude", Account: "personal", Model: "personal-model"}
	got.SetSelectedProfile(ClientClaude, "personal-claude")
	merged, err := Merge(got, team)
	if err != nil {
		t.Fatal(err)
	}
	if merged.SelectedProfile(ClientClaude) != "personal-claude" || merged.SelectedProfile(ClientCodex) != "" {
		t.Fatalf("merge replaced a personal client selection: %#v", merged.Clients)
	}

	exported, err := Export(merged)
	if err != nil {
		t.Fatal(err)
	}
	exportedTeam, err := Parse(exported)
	if err != nil {
		t.Fatal(err)
	}
	if exportedTeam.Recommendations[ClientClaude].Profile != "personal-claude" || exportedTeam.Recommendations[ClientCodex].Profile != "team-codex" {
		t.Fatalf("exported recommendations = %#v", exportedTeam.Recommendations)
	}
}

func TestParseConfigurationManifestAndMergePreservesPersonalState(t *testing.T) {
	raw := []byte(`version = 6

[accounts.team]
label = "Team Gateway"

[accounts.team.endpoints]
openai_responses = "https://gateway.test/v1"
anthropic = "https://gateway.test"

[profiles.team]
label = "Team Gateway"
account = "team"
model = "claude-team"
`)
	m, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.SetClientActivation(ClientClaude, true, "/personal/claude", nil)
	cfg.SetSelectedProfile(ClientClaude, "personal")
	cfg.Accounts["personal"] = Account{Label: "Personal", Endpoints: Endpoints{Anthropic: "https://personal.test"}}
	cfg.Profiles["personal"] = Profile{Label: "Personal", Account: "personal", Model: "claude-personal"}
	got, err := Merge(cfg, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedProfile(ClientClaude) != "personal" {
		t.Fatalf("personal selection changed: %#v", got.Clients)
	}
	if got.Clients[ClientClaude].Executable != "/personal/claude" {
		t.Fatalf("personal adapter changed: %#v", got.Clients)
	}
	if got.Profiles["team"].Label != "Team Gateway" {
		t.Fatalf("imported profile missing: %#v", got.Profiles)
	}
}

func TestMergeRejectsConflictingExistingAccountWithoutMutatingLocalConfig(t *testing.T) {
	team, err := Parse([]byte(`version = 6

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team-profile]
label = "TeamProfile"
account = "team"
model = "claude-team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Personal Gateway", Endpoints: Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "team", Model: "claude-local"}

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
	team, err := Parse([]byte(`version = 6

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[profiles.shared]
label = "Team Model"
account = "team"
model = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Profiles["shared"] = Profile{Label: "Personal Model", Account: "team", Model: "personal-model"}

	_, err = Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `profile "shared" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if got := cfg.Profiles["shared"].Model; got != "personal-model" {
		t.Fatalf("conflicting merge mutated active profile model: %q", got)
	}
}

func TestMergeAcceptsEquivalentExistingIdentityWithoutReplacingLocalState(t *testing.T) {
	team, err := Parse([]byte(`version = 6

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.shared]
label = "TeamProfile"
purpose = "Default agent"
account = "team"
model = "claude-team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{Anthropic: "https://team.example.test/"}}
	cfg.Profiles["shared"] = Profile{Label: "TeamProfile", Purpose: "Default agent", Account: "team", Model: "claude-team"}

	got, err := Merge(cfg, team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["shared"].Model != "claude-team" {
		t.Fatalf("equivalent merge = %#v", got)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://team.example.test/" {
		t.Fatalf("idempotent import should preserve local canonical representation: %#v", got.Accounts["team"])
	}
}

func TestMergeWithOptionsReplacesOnlyExplicitConflictingIdentity(t *testing.T) {
	team, err := Parse([]byte(`version = 6

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[profiles.shared]
label = "Team Model"
account = "team"
model = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Personal Gateway", Endpoints: Endpoints{OpenAIResponses: "https://personal.example.test/v1"}}
	cfg.Profiles["shared"] = Profile{Label: "Personal Model", Account: "team", Model: "personal-model"}

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
	if got.Profiles["shared"].Model != "team-model" {
		t.Fatalf("profile replacement = %#v", got.Profiles["shared"])
	}
}

func TestMergeWithOptionsRejectsUnusedReplacementSelectors(t *testing.T) {
	team, err := Parse([]byte(`version = 6

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team-profile]
label = "TeamProfile"
account = "team"
model = "claude-team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["local"] = Account{Label: "Local", Endpoints: Endpoints{Anthropic: "https://local.example.test"}}
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "local", Model: "claude-local"}

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
	team, err := Parse([]byte(`version = 6

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team]
label = "Team"
account = "team"
model = "claude-team"
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
		Clients: nil,
	}

	_, err = MergeWithOptions(cfg, team, MergeOptions{})
	if err == nil {
		t.Fatal("expected conflict")
	}
	if cfg.Recommendations != nil || cfg.Clients != nil {
		t.Fatalf("rejected merge normalized caller-owned config: %#v", cfg)
	}
}

func TestMergeRejectsNonCanonicalLocalSchemaVersion(t *testing.T) {
	team, err := Parse([]byte(`version = 6
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[profiles.team]
label = "Team"
purpose = "Default agent"
account = "team"
model = "claude-team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Version = 1
	cfg.Accounts["local"] = Account{Label: "Local", Endpoints: Endpoints{Anthropic: "https://local.test"}}
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "local", Model: "claude-local"}
	if _, err := Merge(cfg, team); err == nil || !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("merge error = %v", err)
	}
}

func TestMergeWithOptionsRejectsNonCanonicalConfigurationManifestVersion(t *testing.T) {
	team := Manifest{Version: 99, Profiles: map[string]Profile{"team": {Label: "Team", Account: "team"}}}
	if _, err := MergeWithOptions(NewConfig(), team, MergeOptions{}); err == nil || !strings.Contains(err.Error(), "unsupported configuration manifest version 99") {
		t.Fatalf("unsupported manifest version error = %v", err)
	}
}

func TestMergeDefaultsToFirstImportedProfileWhenNeitherSideChoosesADefault(t *testing.T) {
	team, err := Parse([]byte(`version = 6
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.solo]
label = "Solo"
account = "team"
model = "claude-solo"
`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Merge(NewConfig(), team)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("merge invented a client binding without a recommendation: %#v", got.Clients)
	}
}

func TestMergeRejectsConflictingModelOverrideWithOtherwiseIdenticalProfile(t *testing.T) {
	team, err := Parse([]byte(`version = 6
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"
[profiles.shared]
label = "TeamProfile"
account = "team"
model = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Profiles["shared"] = Profile{Label: "TeamProfile", Account: "team", Model: "personal-model"}

	if _, err := Merge(cfg, team); err == nil || !strings.Contains(err.Error(), `profile "shared" conflicts`) {
		t.Fatalf("model-only conflict error = %v", err)
	}
}

func TestMergeTreatsIdenticalAccountProbesAsEquivalent(t *testing.T) {
	team, err := Parse([]byte(`version = 6
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
model = "claude-team"
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
	cfg.Profiles["local"] = Profile{Label: "Local", Account: "team", Model: "claude-local"}

	got, err := Merge(cfg, team)
	if err != nil {
		t.Fatalf("equivalent account probes must not conflict: %v", err)
	}
	if got.Accounts["team"].AccountProbe.Kind != "dmxapi" {
		t.Fatalf("merged account probe = %#v", got.Accounts["team"].AccountProbe)
	}
}
