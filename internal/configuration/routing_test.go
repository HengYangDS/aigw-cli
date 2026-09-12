package configuration

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeResolutionPreservesConfiguration(t *testing.T) {
	cfg := validConfig()
	want := validConfig()
	resolved, err := cfg.ResolveRuntime(ClientClaude, "")
	if err != nil || resolved.AccountID != "dmx" || resolved.Endpoint != cfg.Accounts["dmx"].Endpoints.Anthropic {
		t.Fatalf("selected Account capability = %#v, %v", resolved, err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatal("runtime resolution changed the supplied configuration")
	}
	empty := Config{}
	if _, err := empty.ResolveRuntime(ClientClaude, ""); err == nil || !reflect.DeepEqual(empty, Config{}) {
		t.Fatalf("empty configuration resolution = %#v, %v", empty, err)
	}
}

func BenchmarkResolveRuntime(b *testing.B) {
	cfg := validConfig()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := cfg.ResolveRuntime(ClientClaude, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func TestResolveRuntimeReturnsAccountEndpointAndModel(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6"] = Profile{Label: "GPT-5.6", Account: "dmx", Client: ClientCodex, Model: "gpt-5.6"}
	cfg.Profiles["claude-opus"] = Profile{Label: "Claude Opus", Account: "dmx", Client: ClientClaude, Model: "claude-opus"}
	cfg.Routes[ClientCodex] = "gpt-5.6"
	cfg.Routes[ClientClaude] = "claude-opus"
	got, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != "gpt-5.6" || got.AccountID != "dmx" || got.Model != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v", got)
	}
	got, err = cfg.ResolveRuntime(ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != "claude-opus" || got.Model != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v", got)
	}
}

func TestClientForProfileRequiresCanonicalScope(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Client: ClientCodex, Model: "gpt-5.6"}
	cfg.Profiles["shared"] = Profile{Label: "Shared", Account: "dmx", Model: "gpt-5.6"}

	client, err := cfg.ClientForProfile("codex")
	if err != nil || client != ClientCodex {
		t.Fatalf("ClientForProfile(codex) = %q, %v", client, err)
	}
	if _, err := cfg.ClientForProfile("shared"); err == nil || !strings.Contains(err.Error(), "does not declare exactly one admitted client") {
		t.Fatalf("unscoped profile error = %v", err)
	}
	if _, err := cfg.ClientForProfile("missing"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}
}

func TestResolveRuntimeRejectsUnknownProfileAccountAndEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Client: ClientCodex, Model: "gpt-test"}
	cfg.Routes[ClientCodex] = "codex"

	if _, err := cfg.ResolveRuntime(ClientCodex, "missing-profile"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}

	orphan := cfg
	orphan.Profiles = map[string]Profile{"codex": {Label: "Codex", Account: "missing-account", Client: ClientCodex, Model: "gpt-test"}}
	orphan.Routes[ClientCodex] = "codex"
	_, err := orphan.ResolveRuntime(ClientCodex, "")
	var accountErr *RuntimeProfileUnknownAccountError
	if !errors.As(err, &accountErr) || accountErr.ProfileID != "codex" || accountErr.AccountID != "missing-account" {
		t.Fatalf("unknown account error = %v", err)
	}

	claudeProfile := cfg.Profiles["codex"]
	claudeProfile.Client = ClientClaude
	claudeProfile.Model = "claude-test"
	cfg.Profiles["claude"] = claudeProfile
	_, err = cfg.ResolveRuntime(ClientClaude, "claude")
	var endpointErr *RuntimeMissingEndpointError
	if !errors.As(err, &endpointErr) || endpointErr.AccountID != "dmx" || endpointErr.Protocol != ProtocolAnthropic {
		t.Fatalf("missing endpoint error = %v", err)
	}

	scoped := cfg
	profile := scoped.Profiles["codex"]
	profile.Client = ClientCodex
	scoped.Profiles["codex"] = profile
	_, err = scoped.ResolveRuntime(ClientClaude, "codex")
	var mismatchErr *RuntimeProfileClientMismatchError
	if !errors.As(err, &mismatchErr) || mismatchErr.ProfileID != "codex" || mismatchErr.ExpectedClient != ClientCodex || mismatchErr.ActualClient != ClientClaude {
		t.Fatalf("profile client mismatch error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsKeepsCapabilityAndChoosesUsableProfiles(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["beta"] = Account{Label: "Beta", Endpoints: Endpoints{Anthropic: "https://beta.test", OpenAIResponses: "https://beta.test/v1"}}
	cfg.Profiles["alpha-claude"] = Profile{Label: "Alpha Claude", Account: "alpha", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["alpha-codex"] = Profile{Label: "Alpha Codex", Account: "alpha", Client: ClientCodex, Model: "gpt-test"}
	cfg.Profiles["beta-claude"] = Profile{Label: "Beta Claude", Account: "beta", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["beta-codex"] = Profile{Label: "Beta Codex", Account: "beta", Client: ClientCodex, Model: "gpt-test"}
	cfg.Routes[ClientCodex] = "alpha-codex"
	cfg.Routes[ClientClaude] = "alpha-claude"
	cfg.Routes[ClientCodex] = "alpha-codex"

	one, err := cfg.SelectRoutesForConnectedAccounts([]string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if one.Routes[ClientClaude] != "beta-claude" || one.Routes[ClientCodex] != "beta-codex" {
		t.Fatalf("one connected Account routes = %#v", one.Routes)
	}
	if len(one.Accounts) != 2 || len(one.Profiles) != 4 {
		t.Fatalf("route selection discarded catalogue capability: %#v", one)
	}

	both, err := cfg.SelectRoutesForConnectedAccounts([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(both.Routes, cfg.Routes) {
		t.Fatalf("usable recommended routes changed: got %#v want %#v", both.Routes, cfg.Routes)
	}

	if _, err := cfg.SelectRoutesForConnectedAccounts([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("unknown connected Account error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsPreservesRecommendedModels(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["preferred"] = Account{Label: "Preferred", Endpoints: Endpoints{Anthropic: "https://preferred.test", OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Accounts["connected"] = Account{Label: "Connected", Endpoints: Endpoints{Anthropic: "https://connected.test", OpenAIResponses: "https://connected.test/v1"}}
	cfg.Profiles["preferred-claude"] = Profile{Label: "Preferred Claude", Account: "preferred", Client: ClientClaude, Model: "claude-fable-5"}
	cfg.Profiles["preferred-codex"] = Profile{Label: "Preferred Codex", Account: "preferred", Client: ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Profiles["connected-claude"] = Profile{Label: "Connected Claude", Account: "connected", Client: ClientClaude, Model: "claude-fable-5"}
	cfg.Profiles["connected-codex-luna"] = Profile{Label: "Connected Codex Luna", Account: "connected", Client: ClientCodex, Model: "gpt-5.6-luna"}
	cfg.Profiles["connected-codex-sol"] = Profile{Label: "Connected Codex Sol", Account: "connected", Client: ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Routes[ClientClaude] = "preferred-claude"
	cfg.Routes[ClientCodex] = "preferred-codex"

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Routes[ClientCodex] != "connected-codex-sol" {
		t.Fatalf("Codex route lost the recommended model: %#v", selected.Routes)
	}
	if selected.Routes[ClientClaude] != "connected-claude" {
		t.Fatalf("Claude route = %q", selected.Routes[ClientClaude])
	}
}

func TestRoutedAccountIDsReturnsUniqueStableActiveAccounts(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["optional"] = Account{Label: "Optional", Endpoints: Endpoints{OpenAIResponses: "https://optional.test/v1"}}
	cfg.Profiles["alpha-claude"] = Profile{Label: "Alpha Claude", Account: "alpha", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["alpha-codex"] = Profile{Label: "Alpha Codex", Account: "alpha", Client: ClientCodex, Model: "gpt-test"}
	cfg.Profiles["optional"] = Profile{Label: "Optional", Account: "optional", Client: ClientCodex, Model: "gpt-optional"}
	cfg.Routes[ClientClaude] = "alpha-claude"
	cfg.Routes[ClientCodex] = "alpha-codex"

	if got := cfg.RoutedAccountIDs(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("RoutedAccountIDs() = %#v", got)
	}
}

func TestSelectRoutesForConnectedAccountsSelectsACompatibleProfileForAnUnselectedClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["claude-only"] = Account{Label: "Claude only", Endpoints: Endpoints{Anthropic: "https://claude.test"}}
	cfg.Accounts["codex"] = Account{Label: "Codex", Endpoints: Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Profiles["claude"] = Profile{Label: "Claude", Account: "claude-only", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "codex", Client: ClientCodex, Model: "gpt-test"}
	cfg.Routes[ClientCodex] = "codex"

	got, err := cfg.SelectRoutesForConnectedAccounts([]string{"claude-only"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes[ClientClaude] != "claude" {
		t.Fatalf("selected routes = %#v", got.Routes)
	}
}

func TestSelectRoutesForConnectedAccountsSkipsProfilesWithoutTheClientEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Profiles["generic"] = Profile{Label: "Generic", Account: "team", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["invalid-codex"] = Profile{Label: "Invalid Codex", Account: "team", Client: ClientCodex, Model: "gpt-test"}

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Routes[ClientCodex] != "" {
		t.Fatalf("profile without Responses endpoint selected for Codex: %#v", selected.Routes)
	}
}

func TestEndpointForRejectsUnknownClientAndMissingProtocolEndpoint(t *testing.T) {
	account := Account{ID: "dmx", Label: "DMXAPI", Endpoints: Endpoints{Anthropic: "https://dmx.test"}}
	if _, err := account.EndpointFor("gemini"); err == nil || !strings.Contains(err.Error(), "unknown client") {
		t.Fatalf("unknown client error = %v", err)
	}
	if _, err := account.EndpointFor(ClientCodex); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
		t.Fatalf("missing OpenAI endpoint error = %v", err)
	}
	if endpoint, err := account.EndpointFor(ClientClaude); err != nil || endpoint != "https://dmx.test" {
		t.Fatalf("EndpointFor = %q, %v", endpoint, err)
	}
}

func TestResolveAccountRejectsProfileWithUnknownAccount(t *testing.T) {
	cfg := NewConfig()
	cfg.Profiles["orphan"] = Profile{Label: "Orphan", Account: "missing"}
	_, _, err := cfg.ResolveAccount("orphan")
	if err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("orphan profile error = %v", err)
	}
}

func TestDomainErrorTextAndProfileSelectionBranches(t *testing.T) {
	if got := endpointProtocolName(ProtocolAnthropic); got != "Anthropic" {
		t.Fatalf("Anthropic protocol name = %q", got)
	}
	if got := endpointProtocolName(ProtocolOpenAIResponses); got != "OpenAI Responses" {
		t.Fatalf("Responses protocol name = %q", got)
	}
	if got := (&RuntimeProfileClientMismatchError{ProfileID: "one", ExpectedClient: "codex", ActualClient: "claude"}).Error(); !strings.Contains(got, "for codex") {
		t.Fatalf("mismatch error = %q", got)
	}
	if got := (&RuntimeProfileUnknownAccountError{ProfileID: "one", AccountID: "missing"}).Error(); !strings.Contains(got, "unknown account") {
		t.Fatalf("account error = %q", got)
	}
	if got := (&RuntimeMissingEndpointError{AccountID: "one", Protocol: EndpointProtocol("future")}).Error(); !strings.Contains(got, "future endpoint") {
		t.Fatalf("endpoint error = %q", got)
	}

	cfg := NewConfig()
	cfg.Accounts["generic"] = Account{Label: "Generic", Endpoints: Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["a-skip"] = Profile{Label: "Skip", Account: "generic", Client: ClientClaude}
	cfg.Profiles["b-endpoint"] = Profile{Label: "Endpoint", Account: "generic", Client: ClientClaude, Model: "claude-endpoint"}
	if got := cfg.FirstProfileForClient(ClientCodex); got != "" {
		t.Fatalf("Codex profile = %q", got)
	}
	if got := cfg.FirstProfileForClient(ClientClaude); got != "a-skip" {
		t.Fatalf("Claude profile = %q", got)
	}
}
