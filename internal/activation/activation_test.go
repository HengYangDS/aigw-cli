package activation

import (
	"testing"

	"aigw-cli/internal/configuration"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
)

type unobservedWritableStore struct{}

func (unobservedWritableStore) Get(string) (string, error) { panic("unselected Token value was read") }
func (unobservedWritableStore) Set(string, string) error   { panic("Token was written") }
func (unobservedWritableStore) Delete(string) error        { panic("Token was deleted") }
func (unobservedWritableStore) Exists(string) (bool, error) {
	panic("unselected native Token metadata was observed")
}

func TestAssessActivationChoosesOneCompatibleEnvironmentAccount(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["ucloud"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://ucloud.test"}}
	cfg.Accounts["dmx"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://dmx.test"}}
	cfg.Routes["ucloud-claude"] = configuration.Route{Account: "ucloud", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.Routes["dmx-claude"] = configuration.Route{Account: "dmx", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(configuration.ClientClaude, "ucloud-claude")
	cfg.SetSelectedRoute(configuration.ClientClaude, "dmx-claude")
	store := secrets.NewEnvironmentStore(func(string) string { return "" })
	got := AssessActivation(cfg, store)
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextAction != "set environment variable "+secrets.EnvironmentKey("dmx") {
		t.Fatalf("selected Account activation = %+v", got)
	}

	store = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("dmx") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, store)
	if got.NextAction != "aigw sync" {
		t.Fatalf("connected Account activation = %+v", got)
	}

	store = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("ucloud") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, store)
	if got.NextAction != "set environment variable "+secrets.EnvironmentKey("dmx") {
		t.Fatalf("recommended Account replaced an explicit selection: %+v", got)
	}
}

func TestAssessActivationListsUnselectedCompatibleEnvironmentAccounts(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["ucloud"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://ucloud.test"}}
	cfg.Accounts["dmx"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://dmx.test"}}
	cfg.Routes["ucloud-claude"] = configuration.Route{Account: "ucloud", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.Routes["dmx-claude"] = configuration.Route{Account: "dmx", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(configuration.ClientClaude, "ucloud-claude")
	recommendation := cfg.Recommendations[configuration.ClientClaude]
	recommendation.Alternatives = []configuration.ClientSelection{{Route: "dmx-claude"}}
	cfg.Recommendations[configuration.ClientClaude] = recommendation

	got := AssessActivation(cfg, secrets.NewEnvironmentStore(func(string) string { return "" }))
	want := "Set one compatible Account variable: " + secrets.EnvironmentKey("dmx") + " or " + secrets.EnvironmentKey("ucloud")
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextAction != want {
		t.Fatalf("unselected alternatives = %+v, want %q", got, want)
	}
}

func TestAssessActivationDoesNotObserveUnselectedNativeCredentials(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(configuration.ClientClaude, "claude")
	got := AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextAction != "Choose one compatible Account: aigw rotate team" || !got.CredentialPrerequisite {
		t.Fatalf("native backend activation = %+v", got)
	}

	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	got = AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 1 || got.State != "" || got.NextAction != "" {
		t.Fatalf("enabled client activation = %+v", got)
	}
}

func TestAssessActivationSelectedWritableAccountRequiresItsToken(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	store := secrets.NewMemoryStore()

	got := AssessActivation(cfg, store)
	if got.NextAction != "run `aigw rotate team`" || !got.CredentialPrerequisite {
		t.Fatalf("selected Account without Token = %+v", got)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	got = AssessActivation(cfg, store)
	if got.NextAction != "aigw sync" || got.CredentialPrerequisite {
		t.Fatalf("selected Account with Token = %+v", got)
	}
}

func TestAssessActivationSeparatesEnabledIntentFromDeferredProjection(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["hermes"] = configuration.Route{Account: "team", Model: "model", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientHermes, "hermes")
	cfg.SetClientActivation(configuration.ClientHermes, true, "", nil)

	got := AssessActivation(cfg, unobservedWritableStore{})
	want := "Install Hermes if needed, then run `aigw sync`"
	if got.EnabledClients != 1 || got.State != domainreadiness.Deferred || got.NextAction != want || got.ProjectionPrerequisites[configuration.ClientHermes] != want {
		t.Fatalf("selected but unprojected client = %+v", got)
	}

	cfg.SetSelectedRoute(configuration.ClientCodex, "hermes")
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{"/opt/codex/config.toml"})
	got = AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 2 || got.State != "" || got.NextAction != "" || got.ProjectionPrerequisites[configuration.ClientHermes] != want {
		t.Fatalf("mixed projected and deferred clients = %+v", got)
	}
}
