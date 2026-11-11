package domain

import (
	"errors"
	"testing"
)

// credentialViewOf is the view that the store decodes from cfg's file.
func credentialViewOf(cfg Config) CredentialView {
	view := CredentialView{Clients: map[string]CredentialClient{}}
	for _, id := range cfg.ProviderIDs() {
		provider := CredentialProvider{ID: id, Endpoints: cfg.Providers[id].Endpoints}
		for _, ref := range cfg.ProviderModelRefs(id) {
			provider.Models = append(provider.Models, modelAdmittedProtocols(cfg.Models[ref]))
		}
		view.Providers = append(view.Providers, provider)
	}
	for client, binding := range cfg.Clients {
		view.Clients[client] = CredentialClient{Enabled: binding.Enabled, Authentication: binding.Authentication}
	}
	return view
}

// TestCredentialViewServesEveryFingerprintThatTheProgramProjects resolves
// every model on every protocol that each client can use, as the program
// does before it writes the client's credential command, and requires the
// reader's view to name the same provider for that command's fingerprint.
func TestCredentialViewServesEveryFingerprintThatTheProgramProjects(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["gateway"] = Provider{Label: "Gateway", Endpoints: Endpoints{
		Anthropic: "https://gateway.test/", Responses: "https://gateway.test/v1", Chat: "https://gateway.test/chat/v1/",
	}}
	cfg.Providers["other"] = Provider{Label: "Other", Endpoints: Endpoints{Responses: "https://other.test/v1"}}
	for _, model := range []Model{
		{Provider: "gateway", ID: "every", Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {}, ProtocolResponses: {}, ProtocolChat: {}}},
		{Provider: "other", ID: "responses", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}},
	} {
		cfg.Models[model.Ref()] = model
	}
	for _, client := range cfg.Admission().IDs() {
		cfg.SetClientActivation(client, true, "/opt/"+client, nil)
	}
	view := credentialViewOf(cfg)
	served := 0
	for _, client := range cfg.Admission().IDs() {
		spec, _ := cfg.Admission().Spec(client)
		for _, ref := range cfg.ModelRefs() {
			for _, protocol := range knownProtocols {
				runtime, err := cfg.ResolveModelProtocol(client, ref, protocol)
				if err != nil {
					continue
				}
				served++
				if provider, err := view.TokenProvider(spec, runtime.CredentialProjectionFingerprint(client)); err != nil || provider != runtime.ProviderID {
					t.Errorf("%s on %s %s = %q, %v; want %q", client, ref, protocol, provider, err, runtime.ProviderID)
				}
			}
		}
	}
	if served != 8 {
		t.Fatalf("the program projects %d fingerprints, want 8", served)
	}
}

// TestCredentialViewRefusesWhatItCannotServe refuses a disabled client, a
// client that authenticates itself or with an unknown authentication, and
// an endpoint that no model of its provider declares, by name.
func TestCredentialViewRefusesWhatItCannotServe(t *testing.T) {
	spec := ClientSpec{ID: ClientCodex, EndpointProtocols: []EndpointProtocol{ProtocolResponses}}
	fingerprint := upstreamIdentity(ClientCodex, "gateway", "https://gateway.test/v1")
	view := func(client CredentialClient, models ...[]EndpointProtocol) CredentialView {
		return CredentialView{
			Providers: []CredentialProvider{{ID: "gateway", Endpoints: Endpoints{Responses: "https://gateway.test/v1"}, Models: models}},
			Clients:   map[string]CredentialClient{ClientCodex: client},
		}
	}
	responses := []EndpointProtocol{ProtocolResponses}
	if provider, err := view(CredentialClient{Enabled: true}, responses).TokenProvider(spec, fingerprint); err != nil || provider != "gateway" {
		t.Fatalf("the served client = %q, %v", provider, err)
	}
	for name, test := range map[string]struct {
		view CredentialView
		want error
	}{
		"disabled":                  {view(CredentialClient{Authentication: AuthenticationProviderToken}, responses), ErrCredentialClientDisabled},
		"absent":                    {CredentialView{}, ErrCredentialClientDisabled},
		"authenticated by itself":   {view(CredentialClient{Enabled: true, Authentication: AuthenticationClientNative}, responses), ErrCredentialClientOwned},
		"no model on the protocol":  {view(CredentialClient{Enabled: true}, []EndpointProtocol{ProtocolChat}), errCredentialUnmatched},
		"a model without protocols": {view(CredentialClient{Enabled: true}, []EndpointProtocol{}), errCredentialUnmatched},
		"no model":                  {view(CredentialClient{Enabled: true}), errCredentialUnmatched},
	} {
		if provider, err := test.view.TokenProvider(spec, fingerprint); !errors.Is(err, test.want) || provider != "" {
			t.Errorf("%s = %q, %v; want %v", name, provider, err, test.want)
		}
	}
	_, err := view(CredentialClient{Enabled: true, Authentication: "device-code"}, responses).TokenProvider(spec, fingerprint)
	if unknown, ok := errors.AsType[*UnknownAuthenticationError](err); !ok || unknown.Authentication != "device-code" || err.Error() != `the client's authentication "device-code" is unknown to this reader` {
		t.Fatalf("an unknown authentication = %v", err)
	}
}
