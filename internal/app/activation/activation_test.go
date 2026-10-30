package activation

import (
	"errors"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	domainreadiness "aigw-cli/internal/app/readiness"
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
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

type deniedMetadataStore struct{ secret.Store }

func (deniedMetadataStore) Exists(string) (bool, error) {
	return false, errors.New("metadata denied")
}

func TestAssessActivationChoosesOneCompatibleEnvironmentAccount(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["ucloud"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://ucloud.test"}}
	cfg.Accounts["dmx"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://dmx.test"}}
	cfg.Routes["ucloud-claude"] = domain.Route{Account: "ucloud", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["dmx-claude"] = domain.Route{Account: "dmx", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(claude.ID, "ucloud-claude")
	cfg.SetSelectedRoute(claude.ID, "dmx-claude", "")
	store := secret.NewEnvironmentStore(func(string) string { return "" })
	got := AssessActivation(cfg, store)
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != "set environment variable "+secret.EnvironmentKey("dmx") {
		t.Fatalf("selected Account activation = %+v", got)
	}

	store = secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("dmx") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "aigw use --for claude dmx-claude" {
		t.Fatalf("connected Account activation = %+v", got)
	}

	store = secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("ucloud") {
			return "available-token"
		}
		return ""
	})
	got = AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "set environment variable "+secret.EnvironmentKey("dmx") {
		t.Fatalf("recommended Account replaced an explicit selection: %+v", got)
	}
}

func TestAssessActivationSeparatesCapabilityAndSelectionPrerequisites(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	store := secret.NewMemoryStore()
	capability := AssessActivation(cfg, store)
	if capability.CapabilityPrerequisite != "aigw setup" || capability.SelectionPrerequisite != "" || capability.NextActionFor(nil) != "aigw setup" {
		t.Fatalf("missing capability = %+v", capability)
	}

	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	selection := AssessActivation(cfg, store)
	if selection.CapabilityPrerequisite != "" || selection.SelectionPrerequisite != "aigw use --help" || selection.VerificationPrerequisite != "" || selection.NextActionFor(nil) != "aigw use --help" {
		t.Fatalf("missing client selection = %+v", selection)
	}
}

func TestAssessActivationDoesNotRequireTokenBackendBeforeItsUse(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1"}}
	cfg.Routes["codex"] = domain.Route{Account: "team", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	selection := AssessActivation(cfg, nil)
	if selection.State != domainreadiness.Deferred || selection.NextActionFor(nil) != "aigw use --help" {
		t.Fatalf("missing Token backend blocked selection before any Account was chosen: %+v", selection)
	}

	cfg.SetSelectedRoute(codex.ID, "codex", "")
	for _, test := range []struct {
		authentication domain.Authentication
		provider       string
		command        string
	}{
		{authentication: domain.AuthenticationClientNative, provider: "native-provider"},
		{authentication: domain.AuthenticationAccountToken, command: filepath.Join(t.TempDir(), "external-credential")},
	} {
		binding := cfg.Clients[codex.ID]
		binding.Authentication, binding.ModelProvider, binding.CredentialCommand = test.authentication, test.provider, test.command
		cfg.Clients[codex.ID] = binding
		for _, store := range []secret.Store{nil, unobservedWritableStore{}} {
			decision := AssessActivation(cfg, store)
			if decision.State != domainreadiness.Deferred || decision.NextActionFor(nil) != "aigw use --for codex codex" || len(decision.ClientCredentialPrerequisites) != 0 {
				t.Fatalf("independent credential selection required an unused AIGW Token backend: %+v", decision)
			}
		}
	}
}

func TestNextActionAfterAccountConnectionPreservesExplicitSelection(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["hermes"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(claude.ID, "claude")
	cfg.SetRecommendedRoute(hermes.ID, "hermes")
	if got := NextActionAfterAccountConnection(cfg, "team"); got != "aigw use --for claude claude" || cfg.SelectedRoute(claude.ID) != "" {
		t.Fatalf("Account connection mutated or skipped selection: action=%q bindings=%#v", got, cfg.Clients)
	}

	cfg.SetSelectedRoute(claude.ID, "claude", "")
	if got := NextActionAfterAccountConnection(cfg, "team"); got != "aigw use --for claude claude" {
		t.Fatalf("unprojected selected client action = %q", got)
	}
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	if got := NextActionAfterAccountConnection(cfg, "team"); got != "aigw check" {
		t.Fatalf("projected selected client action = %q", got)
	}
	cfg.SetSelectedRoute(claude.ID, "", "")
	cfg.Recommendations = map[string]domain.ClientRecommendation{}
	if got := NextActionAfterAccountConnection(cfg, "team"); got != "aigw use --help" {
		t.Fatalf("Account without a recommendation action = %q", got)
	}
}

func TestNextActionAfterAccountConnectionKeepsRequiredProtocolExplicit(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	cfg.Routes["multi"] = domain.Route{Account: "team", Model: "model", Interfaces: map[domain.EndpointProtocol][]domain.Capability{
		domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {},
	}}
	cfg.Recommendations[hermes.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{
		Route: "multi", Protocol: domain.ProtocolOpenAIResponses,
	}}
	want := "aigw use --for hermes --protocol openai_responses multi"
	if got := NextActionAfterAccountConnection(cfg, "team"); got != want {
		t.Fatalf("multi-protocol action = %q, want %q", got, want)
	}
}

func TestAssessActivationListsUnselectedCompatibleEnvironmentAccounts(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["ucloud"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://ucloud.test"}}
	cfg.Accounts["dmx"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://dmx.test"}}
	cfg.Routes["ucloud-claude"] = domain.Route{Account: "ucloud", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["dmx-claude"] = domain.Route{Account: "dmx", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(claude.ID, "ucloud-claude")
	recommendation := cfg.Recommendations[claude.ID]
	recommendation.Alternatives = []domain.ClientSelection{{Route: "dmx-claude"}}
	cfg.Recommendations[claude.ID] = recommendation

	got := AssessActivation(cfg, secret.NewEnvironmentStore(func(string) string { return "" }))
	want := "Set one compatible Account variable: " + secret.EnvironmentKey("dmx") + " or " + secret.EnvironmentKey("ucloud") + ", then run `aigw sync`"
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != want {
		t.Fatalf("unselected alternatives = %+v, want %q", got, want)
	}
}

func TestAssessActivationDoesNotObserveUnselectedNativeCredentials(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetRecommendedRoute(claude.ID, "claude")
	got := AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != "Choose one compatible Account: aigw rotate team" || got.CredentialPrerequisite == "" {
		t.Fatalf("native backend activation = %+v", got)
	}

	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	got = AssessActivation(cfg, selectedMetadataStore{})
	if got.EnabledClients != 1 || got.State != "" || got.VerificationPrerequisite != "aigw check" {
		t.Fatalf("enabled client activation = %+v", got)
	}
}

func TestAssessActivationSelectedWritableAccountRequiresItsToken(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	store := secret.NewMemoryStore()

	got := AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "run `aigw rotate team`" || got.CredentialPrerequisite == "" {
		t.Fatalf("selected Account without Token = %+v", got)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	got = AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "aigw use --for claude claude" || got.CredentialPrerequisite != "" {
		t.Fatalf("selected Account with Token = %+v", got)
	}
}

func TestAssessActivationDoesNotRecommendNoOpSyncBeforeClientActivation(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	store := secret.NewMemoryStore()
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}

	decision := AssessActivation(cfg, store)
	got := decision.NextActionFor(nil)
	want := "aigw use --for claude claude"
	if got != want {
		t.Fatalf("next action before client activation = %q, want %q", got, want)
	}
}

func TestAssessActivationDoesNotInstallAClientWhoseSelectedTokenIsMissing(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["missing"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://missing.test"}}
	cfg.Accounts["connected"] = domain.Account{Endpoints: domain.Endpoints{OpenAIResponses: "https://connected.test/v1"}}
	cfg.Routes["claude"] = domain.Route{Account: "missing", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["codex"] = domain.Route{Account: "connected", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetRecommendedRoute(codex.ID, "codex")
	store := secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("connected") {
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
		{State: domainreadiness.Deferred, NextAction: "set environment variable " + secret.EnvironmentKey("missing")},
		{State: domainreadiness.Deferred, NextAction: "aigw use --for codex codex"},
	}
	if action := got.NextActionFor(clients); action != want {
		t.Fatalf("client inspection displaced the usable continuation: %q, want %q", action, want)
	}
}

func TestAssessActivationDoesNotVerifyWithoutCredentialMetadata(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)

	for _, test := range []struct {
		name  string
		store secret.Store
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
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)

	got := AssessActivation(cfg, secret.NewMemoryStore())
	if got.State != "" || got.NextActionFor(nil) != "run `aigw rotate team`" || got.VerificationPrerequisite != "" {
		t.Fatalf("missing selected Token admitted verification: %+v", got)
	}
}

func TestAssessActivationSeparatesEnabledIntentFromDeferredProjection(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["hermes"] = domain.Route{Account: "team", Model: "model", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(hermes.ID, "hermes", "")
	cfg.SetClientActivation(hermes.ID, true, "", nil)

	got := AssessActivation(cfg, selectedMetadataStore{})
	want := "Install Hermes if needed, then run `aigw sync`"
	if got.EnabledClients != 1 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != want || got.ProjectionPrerequisites[hermes.ID] != want {
		t.Fatalf("selected but unprojected client = %+v", got)
	}

	cfg.SetSelectedRoute(codex.ID, "hermes", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{"/opt/codex/config.toml"})
	got = AssessActivation(cfg, selectedMetadataStore{})
	if got.EnabledClients != 2 || got.State != "" || got.NextActionFor(nil) != want || got.ProjectionPrerequisites[hermes.ID] != want {
		t.Fatalf("mixed projected and deferred clients = %+v", got)
	}
}

func TestAssessActivationSeparatesMissingTokenFromDeferredProjection(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["hermes"] = domain.Route{Account: "team", Model: "model", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(hermes.ID, "hermes", "")
	cfg.SetClientActivation(hermes.ID, true, "", nil)

	missing := secret.NewEnvironmentStore(func(string) string { return "" })
	got := AssessActivation(cfg, missing)
	wantCredential := "set environment variable " + secret.EnvironmentKey("team")
	wantProjection := "Install Hermes if needed, then run `aigw sync`"
	if got.State != domainreadiness.Deferred || got.ClientCredentialPrerequisites[hermes.ID] != wantCredential || got.NextActionFor(nil) != wantCredential || got.ProjectionPrerequisites[hermes.ID] != wantProjection {
		t.Fatalf("missing Token and client = %+v", got)
	}

	connected := secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("team") {
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
			activation: Activation{EnabledClients: 1, State: domainreadiness.Deferred, ProjectionPrerequisites: map[string]string{claude.ID: "aigw sync"}, admission: builtinclients.Admission()},
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
			ClientCredentialPrerequisites: map[string]string{claude.ID: rotateClaude},
			ProjectionPrerequisites:       map[string]string{claude.ID: installClaude, codex.ID: installCodex},
			admission:                     builtinclients.Admission(),
		}, installCodex},
		{"missing Token before its projection", Activation{
			ClientCredentialPrerequisites: map[string]string{claude.ID: rotateClaude},
			ProjectionPrerequisites:       map[string]string{claude.ID: installClaude},
			admission:                     builtinclients.Admission(),
		}, rotateClaude},
		{"unavailable credential metadata", Activation{
			ProjectionPrerequisites: map[string]string{claude.ID: installClaude},
			observedCredentials:     map[string]credentialObservation{"team": {err: errors.New("metadata denied")}},
			admission:               builtinclients.Admission(),
		}, "aigw doctor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.activation.NextActionFor(nil); got != test.want {
				t.Fatalf("next action = %q, want %q", got, test.want)
			}
		})
	}
}
