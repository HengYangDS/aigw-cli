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

func TestAssessActivationChoosesOneCompatibleEnvironmentProvider(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["ucloud"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://ucloud.test"}}
	cfg.Providers["dmx"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://dmx.test"}}
	cfg.Models[domain.NewModelRef("ucloud", "fable")] = domain.Model{Provider: "ucloud", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models[domain.NewModelRef("dmx", "fable")] = domain.Model{Provider: "dmx", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("ucloud", "fable")}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("dmx", "fable"), "")
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
	if got.NextActionFor(nil) != "aigw use claude dmx/fable" {
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

	// A provider without models needs a model selected, not setup.
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	const choose = "aigw use <client> <provider/model>"
	if selection := AssessActivation(cfg, store); selection.CapabilityPrerequisite != "" || selection.NextActionFor(nil) != choose {
		t.Fatalf("a provider without models = %+v", selection)
	}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	selection := AssessActivation(cfg, store)
	if selection.CapabilityPrerequisite != "" || selection.SelectionPrerequisite != choose || selection.VerificationPrerequisite != "" || selection.NextActionFor(nil) != choose {
		t.Fatalf("missing client selection = %+v", selection)
	}
}

func TestAssessActivationDoesNotRequireTokenBackendBeforeItsUse(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Responses: "https://team.test/v1"}}
	cfg.Models[domain.NewModelRef("team", "gpt")] = domain.Model{Provider: "team", Family: "gpt", ID: "gpt", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	selection := AssessActivation(cfg, nil)
	if selection.State != domainreadiness.Deferred || selection.NextActionFor(nil) != "aigw use <client> <provider/model>" {
		t.Fatalf("missing Token backend blocked selection before any Account was chosen: %+v", selection)
	}

	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("team", "gpt"), "")
	for _, test := range []struct {
		authentication domain.Authentication
		provider       string
		command        string
	}{
		{authentication: domain.AuthenticationClientNative, provider: "native-provider"},
		{authentication: domain.AuthenticationProviderToken, command: filepath.Join(t.TempDir(), "external-credential")},
	} {
		binding := cfg.Clients[codex.ID]
		binding.Authentication, binding.ModelProvider, binding.CredentialCommand = test.authentication, test.provider, test.command
		cfg.Clients[codex.ID] = binding
		for _, store := range []secret.Store{nil, unobservedWritableStore{}} {
			decision := AssessActivation(cfg, store)
			if decision.State != domainreadiness.Deferred || decision.NextActionFor(nil) != "aigw use codex team/gpt" || len(decision.ClientCredentialPrerequisites) != 0 {
				t.Fatalf("independent credential selection required an unused AIGW Token backend: %+v", decision)
			}
		}
	}
}

func TestNextActionAfterProviderConnectionPreservesExplicitSelection(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("team", "fable")}}
	cfg.Recommendations[hermes.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("team", "fable")}}
	if got := NextActionAfterProviderConnection(cfg, "team"); got != "aigw use claude team/fable" || !cfg.SelectedModel(claude.ID).IsZero() {
		t.Fatalf("Account connection mutated or skipped selection: action=%q bindings=%#v", got, cfg.Clients)
	}

	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "fable"), "")
	if got := NextActionAfterProviderConnection(cfg, "team"); got != "aigw use claude team/fable" {
		t.Fatalf("unprojected selected client action = %q", got)
	}
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	if got := NextActionAfterProviderConnection(cfg, "team"); got != "aigw check" {
		t.Fatalf("projected selected client action = %q", got)
	}
	cfg.SetSelectedModel(claude.ID, domain.ModelRef{}, "")
	cfg.Recommendations = map[string]domain.ClientRecommendation{}
	if got := NextActionAfterProviderConnection(cfg, "team"); got != "aigw use --help" {
		t.Fatalf("Account without a recommendation action = %q", got)
	}
}

func TestNextActionAfterProviderConnectionKeepsRequiredProtocolExplicit(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{
		Anthropic: "https://team.test", Responses: "https://team.test/v1",
	}}
	cfg.Models[domain.NewModelRef("team", "model")] = domain.Model{Provider: "team", Family: "model", ID: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{
		domain.ProtocolAnthropic: {}, domain.ProtocolResponses: {},
	}}
	cfg.Recommendations[hermes.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{
		Model: domain.NewModelRef("team", "model"), Protocol: domain.ProtocolResponses,
	}}
	want := "aigw use hermes team/model --protocol responses"
	if got := NextActionAfterProviderConnection(cfg, "team"); got != want {
		t.Fatalf("multi-protocol action = %q, want %q", got, want)
	}
}

func TestAssessActivationListsUnselectedCompatibleEnvironmentProviders(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["ucloud"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://ucloud.test"}}
	cfg.Providers["dmx"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://dmx.test"}}
	cfg.Models[domain.NewModelRef("ucloud", "fable")] = domain.Model{Provider: "ucloud", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models[domain.NewModelRef("dmx", "fable")] = domain.Model{Provider: "dmx", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("ucloud", "fable")}}
	recommendation := cfg.Recommendations[claude.ID]
	recommendation.Alternatives = []domain.ClientSelection{{Model: domain.NewModelRef("dmx", "fable")}}
	cfg.Recommendations[claude.ID] = recommendation

	got := AssessActivation(cfg, secret.NewEnvironmentStore(func(string) string { return "" }))
	want := "Set one compatible provider's variable: " + secret.EnvironmentKey("dmx") + " or " + secret.EnvironmentKey("ucloud") + ", then run `aigw sync`"
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != want {
		t.Fatalf("unselected alternatives = %+v, want %q", got, want)
	}
}

func TestAssessActivationDoesNotObserveUnselectedNativeCredentials(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("team", "fable")}}
	got := AssessActivation(cfg, unobservedWritableStore{})
	if got.EnabledClients != 0 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != "Choose one compatible provider: aigw provider rotate team" || got.CredentialPrerequisite == "" {
		t.Fatalf("native backend activation = %+v", got)
	}

	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "fable"), "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	got = AssessActivation(cfg, selectedMetadataStore{})
	if got.EnabledClients != 1 || got.State != "" || got.VerificationPrerequisite != "aigw check" {
		t.Fatalf("enabled client activation = %+v", got)
	}
}

func TestAssessActivationSelectedWritableProviderRequiresItsToken(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "fable"), "")
	store := secret.NewMemoryStore()

	got := AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "run `aigw provider rotate team`" || got.CredentialPrerequisite == "" {
		t.Fatalf("selected Account without Token = %+v", got)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	got = AssessActivation(cfg, store)
	if got.NextActionFor(nil) != "aigw use claude team/fable" || got.CredentialPrerequisite != "" {
		t.Fatalf("selected Account with Token = %+v", got)
	}
}

func TestAssessActivationDoesNotRecommendNoOpSyncBeforeClientActivation(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "fable"), "")
	store := secret.NewMemoryStore()
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}

	decision := AssessActivation(cfg, store)
	got := decision.NextActionFor(nil)
	want := "aigw use claude team/fable"
	if got != want {
		t.Fatalf("next action before client activation = %q, want %q", got, want)
	}
}

func TestAssessActivationDoesNotInstallAClientWhoseSelectedTokenIsMissing(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["missing"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://missing.test"}}
	cfg.Providers["connected"] = domain.Provider{Endpoints: domain.Endpoints{Responses: "https://connected.test/v1"}}
	cfg.Models[domain.NewModelRef("missing", "fable")] = domain.Model{Provider: "missing", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models[domain.NewModelRef("connected", "gpt")] = domain.Model{Provider: "connected", Family: "gpt", ID: "gpt", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("missing", "fable"), "")
	cfg.Recommendations[codex.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("connected", "gpt")}}
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
		{State: domainreadiness.Deferred, NextAction: "aigw use codex team/gpt"},
	}
	if action := got.NextActionFor(clients); action != want {
		t.Fatalf("client inspection displaced the usable continuation: %q, want %q", action, want)
	}
}

func TestAssessActivationDoesNotVerifyWithoutCredentialMetadata(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "fable"), "")
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
			if got.State != domainreadiness.Unavailable || got.NextActionFor(nil) != "aigw check --depth local" || got.VerificationPrerequisite != "" {
				t.Fatalf("unavailable credential metadata admitted verification: %+v", got)
			}
		})
	}
}

func TestAssessActivationDoesNotVerifyWithoutSelectedToken(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "fable"), "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)

	got := AssessActivation(cfg, secret.NewMemoryStore())
	if got.State != "" || got.NextActionFor(nil) != "run `aigw provider rotate team`" || got.VerificationPrerequisite != "" {
		t.Fatalf("missing selected Token admitted verification: %+v", got)
	}
}

func TestAssessActivationSeparatesEnabledIntentFromDeferredProjection(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "model")] = domain.Model{Provider: "team", Family: "model", ID: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(hermes.ID, domain.NewModelRef("team", "model"), "")
	cfg.SetClientActivation(hermes.ID, true, "", nil)

	got := AssessActivation(cfg, selectedMetadataStore{})
	want := "Install Hermes if needed, then run `aigw sync`"
	if got.EnabledClients != 1 || got.State != domainreadiness.Deferred || got.NextActionFor(nil) != want || got.ProjectionPrerequisites[hermes.ID] != want {
		t.Fatalf("selected but unprojected client = %+v", got)
	}

	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("team", "model"), "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{"/opt/codex/config.toml"})
	got = AssessActivation(cfg, selectedMetadataStore{})
	if got.EnabledClients != 2 || got.State != "" || got.NextActionFor(nil) != want || got.ProjectionPrerequisites[hermes.ID] != want {
		t.Fatalf("mixed projected and deferred clients = %+v", got)
	}
}

func TestAssessActivationSeparatesMissingTokenFromDeferredProjection(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "model")] = domain.Model{Provider: "team", Family: "model", ID: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(hermes.ID, domain.NewModelRef("team", "model"), "")
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
			clients:    []domainreadiness.Client{{State: domainreadiness.Invalid, NextAction: "aigw sync"}},
			want:       "set selected Account variable",
		},
		{
			name:       "specific route before generic synchronization",
			activation: Activation{EnabledClients: 1, State: domainreadiness.Deferred, ProjectionPrerequisites: map[string]string{claude.ID: "aigw sync"}, admission: builtinclients.Admission()},
			clients:    []domainreadiness.Client{{State: domainreadiness.Deferred, NextAction: "aigw use claude team/selected"}},
			want:       "aigw use claude team/selected",
		},
		{
			name:       "configured verification before deferred projection",
			activation: Activation{EnabledClients: 2},
			clients: []domainreadiness.Client{
				{State: domainreadiness.Deferred, NextAction: "Install Claude, then sync"},
				{State: domainreadiness.Configured, NextAction: "aigw check codex --depth client"},
			},
			want: "aigw check codex --depth client",
		},
		{
			name:       "unclassified failure",
			activation: Activation{EnabledClients: 1, VerificationPrerequisite: "aigw check"},
			clients:    []domainreadiness.Client{{State: domainreadiness.Invalid}},
			want:       "aigw sync",
		},
		{
			name:       "unclassified failure before unrelated suggestion",
			activation: Activation{EnabledClients: 1},
			clients: []domainreadiness.Client{
				{State: domainreadiness.Invalid},
				{State: domainreadiness.Deferred, NextAction: "aigw use codex team/suggested"},
			},
			want: "aigw sync",
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
	rotateClaude := "aigw provider rotate team"
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
		}, "aigw check --depth local"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.activation.NextActionFor(nil); got != test.want {
				t.Fatalf("next action = %q, want %q", got, test.want)
			}
		})
	}
}

// A store that a blocked path keeps from being observed names, as the next
// step, the fix that a client's observation rendered for it, not only the
// local check.
func TestAnUnobservableStoreNamesTheFixThatAClientRendered(t *testing.T) {
	activation := Activation{CredentialPrerequisite: domainreadiness.CredentialBackendRecovery}
	rendered := "Move the file ~/aigw/secrets. If it replaced AIGW's folder, put that folder back from a copy; otherwise store each provider's Token again with `aigw provider rotate <provider>`. Then run `aigw sync`."
	clients := []domainreadiness.Client{{State: domainreadiness.Unavailable, NextAction: rendered}}
	if got := activation.NextActionFor(clients); got != rendered {
		t.Fatalf("the next step = %q", got)
	}
	unclassified := []domainreadiness.Client{{State: domainreadiness.Unavailable, NextAction: domainreadiness.CredentialBackendRecovery}}
	if got := activation.NextActionFor(unclassified); got != domainreadiness.CredentialBackendRecovery {
		t.Fatalf("an unclassified failure = %q", got)
	}
}
