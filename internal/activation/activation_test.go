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

func TestAssessActivationDoesNotObserveUnselectedNativeCredentials(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(configuration.ClientClaude, "claude")
	got := AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextAction == "aigw check" || got.NextAction == "" {
		t.Fatalf("native backend activation = %+v", got)
	}

	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	got = AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 1 || got.State != "" || got.NextAction != "" {
		t.Fatalf("enabled client activation = %+v", got)
	}
}
