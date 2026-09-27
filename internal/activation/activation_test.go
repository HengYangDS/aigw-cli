package activation

import (
	"errors"
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

type deniedMetadataStore struct{ secrets.Store }

func (deniedMetadataStore) Exists(string) (bool, error) {
	return false, errors.New("metadata denied")
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
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != "set environment variable "+secrets.EnvironmentKey("dmx") {
		t.Fatalf("selected Account activation = %+v", got)
	}

	store = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("dmx") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "Install Claude if needed, then run `aigw sync`" {
		t.Fatalf("connected Account activation = %+v", got)
	}

	store = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("ucloud") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "set environment variable "+secrets.EnvironmentKey("dmx") {
		t.Fatalf("recommended Account replaced an explicit selection: %+v", got)
	}
}

func TestAssessActivationSeparatesCapabilityAndSelectionPrerequisites(t *testing.T) {
	cfg := configuration.NewConfig()
	store := secrets.NewMemoryStore()
	capability := AssessActivation(cfg, store)
	if capability.CapabilityPrerequisite != "aigw setup" || capability.SelectionPrerequisite != "" || capability.NextActionFor(nil) != "aigw setup" {
		t.Fatalf("missing capability = %+v", capability)
	}

	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	selection := AssessActivation(cfg, store)
	if selection.CapabilityPrerequisite != "" || selection.SelectionPrerequisite != "aigw use --help" || selection.VerificationPrerequisite != "" || selection.NextActionFor(nil) != "aigw use --help" {
		t.Fatalf("missing client selection = %+v", selection)
	}
}

func TestAssessActivationDoesNotRequireTokenBackendBeforeItsUse(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}}
	cfg.Routes["codex"] = configuration.Route{Account: "team", Model: "gpt", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	selection := AssessActivation(cfg, nil)
	if selection.State != domainreadiness.Deferred || selection.NextActionFor(nil) != "aigw use --help" {
		t.Fatalf("missing Token backend blocked selection before any Account was chosen: %+v", selection)
	}

	cfg.SetSelectedRoute(configuration.ClientCodex, "codex")
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Authentication = configuration.AuthenticationClientNative
	binding.ModelProvider = "native-provider"
	cfg.Clients[configuration.ClientCodex] = binding
	native := AssessActivation(cfg, nil)
	if native.State != domainreadiness.Deferred || native.NextActionFor(nil) != "Install Codex if needed, then run `aigw sync`" {
		t.Fatalf("client-native selection required an unused Token backend: %+v", native)
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
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != want {
		t.Fatalf("unselected alternatives = %+v, want %q", got, want)
	}
}

func TestAssessActivationDoesNotObserveUnselectedNativeCredentials(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(configuration.ClientClaude, "claude")
	got := AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != "Choose one compatible Account: aigw rotate team" || got.CredentialPrerequisite == "" {
		t.Fatalf("native backend activation = %+v", got)
	}

	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	got = AssessActivation(cfg, selectedMetadataStore{})
	if got.EnabledClients != 1 || got.State != "" || got.VerificationPrerequisite != "aigw check" {
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
	if got.NextActionFor(nil) != "run `aigw rotate team`" || got.CredentialPrerequisite == "" {
		t.Fatalf("selected Account without Token = %+v", got)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	got = AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "Install Claude if needed, then run `aigw sync`" || got.CredentialPrerequisite != "" {
		t.Fatalf("selected Account with Token = %+v", got)
	}
}

func TestAssessActivationDoesNotRecommendNoOpSyncBeforeClientActivation(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	store := secrets.NewMemoryStore()
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}

	decision := AssessActivation(cfg, store)
	got := decision.NextActionFor(nil)
	want := "Install Claude if needed, then run `aigw sync`"
	if got != want {
		t.Fatalf("next action before client activation = %q, want %q", got, want)
	}
}

func TestAssessActivationDoesNotInstallAClientWhoseSelectedTokenIsMissing(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["missing"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://missing.test"}}
	cfg.Accounts["connected"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://connected.test/v1"}}
	cfg.Routes["claude"] = configuration.Route{Account: "missing", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.Routes["codex"] = configuration.Route{Account: "connected", Model: "gpt", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetRecommendedRoute(configuration.ClientCodex, "codex")
	store := secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("connected") {
			return "available-token"
		}
		return ""
	})

	got := AssessActivation(cfg, store)
	want := "Install Codex if needed, then run `aigw sync`"
	if got.NextActionFor(nil) != want {
		t.Fatalf("unavailable selected Token concealed a usable client: %+v; want %q", got, want)
	}
	clients := []domainreadiness.Client{
		{State: domainreadiness.Deferred, NextAction: "set environment variable " + secrets.EnvironmentKey("missing")},
		{State: domainreadiness.Deferred, NextAction: "aigw use --for codex codex"},
	}
	if action := got.NextActionFor(clients); action != want {
		t.Fatalf("client inspection displaced the usable continuation: %q, want %q", action, want)
	}
}

func TestAssessActivationDoesNotVerifyWithoutCredentialMetadata(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)

	for _, test := range []struct {
		name  string
		store secrets.Store
	}{
		{name: "no backend"},
		{name: "backend denied", store: deniedMetadataStore{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := AssessActivation(cfg, test.store)
			if got.State != domainreadiness.Unavailable || got.NextActionFor(nil) != "aigw doctor" || got.VerificationPrerequisite != "" {
				t.Fatalf("unavailable credential metadata admitted verification: %+v", got)
			}
		})
	}
}

func TestAssessActivationDoesNotVerifyWithoutSelectedToken(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)

	got := AssessActivation(cfg, secrets.NewMemoryStore())
	if got.State != "" || got.NextActionFor(nil) != "run `aigw rotate team`" || got.VerificationPrerequisite != "" {
		t.Fatalf("missing selected Token admitted verification: %+v", got)
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
	if got.EnabledClients != 1 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != want || got.ProjectionPrerequisites[configuration.ClientHermes] != want {
		t.Fatalf("selected but unprojected client = %+v", got)
	}

	cfg.SetSelectedRoute(configuration.ClientCodex, "hermes")
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{"/opt/codex/config.toml"})
	got = AssessActivation(cfg, selectedMetadataStore{})
	if got.EnabledClients != 2 || got.State != "" || got.NextActionFor(nil) != want || got.ProjectionPrerequisites[configuration.ClientHermes] != want {
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
	if got.State != domainreadiness.Deferred || got.ClientCredentialPrerequisites[configuration.ClientHermes] != wantCredential || got.NextActionFor(nil) != wantCredential || got.ProjectionPrerequisites[configuration.ClientHermes] != wantProjection {
		t.Fatalf("missing Token and client = %+v", got)
	}

	connected := secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("team") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, connected)
	if got.State != domainreadiness.Deferred || got.CredentialPrerequisite != "" || got.NextActionFor(nil) != wantProjection {
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
			activation: Activation{EnabledClients: 2, CredentialPrerequisite: "set selected Account variable"},
			clients:    []domainreadiness.Client{{State: domainreadiness.Invalid, NextAction: "aigw repair"}},
			want:       "set selected Account variable",
		},
		{
			name:       "specific route before generic synchronization",
			activation: Activation{EnabledClients: 1, State: domainreadiness.Deferred, ProjectionPrerequisites: map[string]string{configuration.ClientClaude: "aigw sync"}},
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
			activation: Activation{EnabledClients: 1, VerificationPrerequisite: "aigw check"},
			clients:    []domainreadiness.Client{{State: domainreadiness.Invalid}},
			want:       "aigw repair",
		},
		{
			name:       "unclassified failure before unrelated suggestion",
			activation: Activation{EnabledClients: 1},
			clients: []domainreadiness.Client{
				{State: domainreadiness.Invalid},
				{State: domainreadiness.Deferred, NextAction: "aigw use --for codex suggested-route"},
			},
			want: "aigw repair",
		},
		{
			name:       "configured client without another action",
			activation: Activation{EnabledClients: 1, VerificationPrerequisite: "aigw check"},
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

func TestNextActionForWithoutClientInspectionHonorsKnownPrerequisites(t *testing.T) {
	installClaude := "Install Claude, then sync"
	installCodex := "Install Codex, then sync"
	rotateClaude := "aigw rotate team"
	for _, test := range []struct {
		name       string
		activation Activation
		want       string
	}{
		{"earlier decision", Activation{CapabilityPrerequisite: "aigw setup"}, "aigw setup"},
		{"usable client first", Activation{
			ClientCredentialPrerequisites: map[string]string{configuration.ClientClaude: rotateClaude},
			ProjectionPrerequisites:       map[string]string{configuration.ClientClaude: installClaude, configuration.ClientCodex: installCodex},
		}, installCodex},
		{"missing Token before its projection", Activation{
			ClientCredentialPrerequisites: map[string]string{configuration.ClientClaude: rotateClaude},
			ProjectionPrerequisites:       map[string]string{configuration.ClientClaude: installClaude},
		}, rotateClaude},
		{"unavailable credential metadata", Activation{
			ProjectionPrerequisites: map[string]string{configuration.ClientClaude: installClaude},
			observedCredentials:     map[string]credentialObservation{"team": {err: errors.New("metadata denied")}},
		}, "aigw doctor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.activation.NextActionFor(nil); got != test.want {
				t.Fatalf("next action = %q, want %q", got, test.want)
			}
		})
	}
}
