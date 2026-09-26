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

type selectedMetadataStore struct{}

func (selectedMetadataStore) Get(string) (string, error) { panic("Token value was read") }
func (selectedMetadataStore) Set(string, string) error   { panic("Token was written") }
func (selectedMetadataStore) Delete(string) error        { panic("Token was deleted") }
func (selectedMetadataStore) Exists(account string) (bool, error) {
	if account != "team" {
		panic("unselected native Token metadata was observed")
	}
	return true, nil
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
	got = AssessActivation(cfg, selectedMetadataStore{})
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

	got := AssessActivation(cfg, selectedMetadataStore{})
	want := "Install Hermes if needed, then run `aigw sync`"
	if got.EnabledClients != 1 || got.State != domainreadiness.Deferred || got.NextAction != want || got.ProjectionPrerequisites[configuration.ClientHermes] != want {
		t.Fatalf("selected but unprojected client = %+v", got)
	}

	cfg.SetSelectedRoute(configuration.ClientCodex, "hermes")
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{"/opt/codex/config.toml"})
	got = AssessActivation(cfg, selectedMetadataStore{})
	if got.EnabledClients != 2 || got.State != "" || got.NextAction != "" || got.ProjectionPrerequisites[configuration.ClientHermes] != want {
		t.Fatalf("mixed projected and deferred clients = %+v", got)
	}
}

func TestAssessActivationSeparatesMissingTokenFromDeferredProjection(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["hermes"] = configuration.Route{Account: "team", Model: "model", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientHermes, "hermes")
	cfg.SetClientActivation(configuration.ClientHermes, true, "", nil)

	missing := secrets.NewEnvironmentStore(func(string) string { return "" })
	got := AssessActivation(cfg, missing)
	wantCredential := "set environment variable " + secrets.EnvironmentKey("team")
	wantProjection := "Install Hermes if needed, then run `aigw sync`"
	if got.State != domainreadiness.Deferred || !got.CredentialPrerequisite || got.NextAction != wantCredential || got.ProjectionPrerequisites[configuration.ClientHermes] != wantProjection {
		t.Fatalf("missing Token and client = %+v", got)
	}

	connected := secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("team") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, connected)
	if got.State != domainreadiness.Deferred || got.CredentialPrerequisite || got.NextAction != wantProjection {
		t.Fatalf("connected Account with deferred client = %+v", got)
	}
}

func TestNextActionForUsesOneOrderedReadinessDecision(t *testing.T) {
	for _, test := range []struct {
		name       string
		activation Activation
		clients    []domainreadiness.Client
		want       string
	}{
		{
			name:       "earlier credential prerequisite",
			activation: Activation{EnabledClients: 2, NextAction: "set selected Account variable", CredentialPrerequisite: true},
			clients:    []domainreadiness.Client{{State: domainreadiness.Invalid, NextAction: "aigw repair"}},
			want:       "set selected Account variable",
		},
		{
			name:       "specific route before generic synchronization",
			activation: Activation{State: domainreadiness.Deferred, NextAction: "aigw sync"},
			clients:    []domainreadiness.Client{{State: domainreadiness.Deferred, NextAction: "aigw use --for claude selected-route"}},
			want:       "aigw use --for claude selected-route",
		},
		{
			name:       "configured verification before deferred projection",
			activation: Activation{EnabledClients: 2},
			clients: []domainreadiness.Client{
				{State: domainreadiness.Deferred, NextAction: "Install Claude, then sync"},
				{State: domainreadiness.Configured, NextAction: "aigw verify --for codex"},
			},
			want: "aigw verify --for codex",
		},
		{
			name:       "unclassified failure",
			activation: Activation{EnabledClients: 1},
			clients:    []domainreadiness.Client{{State: domainreadiness.Invalid}},
			want:       "aigw repair",
		},
		{
			name:       "configured client without another action",
			activation: Activation{EnabledClients: 1},
			clients:    []domainreadiness.Client{{State: domainreadiness.Configured}},
			want:       "aigw check",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.activation.NextActionFor(test.clients); got != test.want {
				t.Fatalf("next action = %q, want %q", got, test.want)
			}
		})
	}
}
