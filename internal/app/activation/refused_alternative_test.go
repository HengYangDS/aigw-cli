package activation

import (
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// observedSlots is a credential store that answers the observation of each
// Token slot from a table, as the system keyring answers: a slot of the table
// is there, and whether AIGW can use it is the table's answer.
type observedSlots struct {
	secret.Store
	slots map[string]secret.TokenObservation
}

func (store observedSlots) Exists(account string) (bool, error) {
	return store.slots[account].Present, nil
}

func (store observedSlots) ObserveToken(account string) secret.TokenObservation {
	return store.slots[account]
}

// refusedSlot is the observation of a provider's Keychain item that macOS
// refuses AIGW, with the refusal that owns its title and its fix.
func refusedSlot(provider string) (secret.TokenObservation, *secret.KeychainItemRefusedError) {
	refusal := &secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: provider, Status: -25293}
	return secret.TokenObservation{Present: true, Refusal: refusal}, refusal
}

// recommending returns a configuration without a selection or an enabled
// client, whose recommendation for Claude Code offers a model of each
// provider, the first as its primary one.
func recommending(providers ...string) domain.Config {
	cfg := teamRecommendation()
	delete(cfg.Providers, "team")
	delete(cfg.Models, domain.NewModelRef("team", "fable"))
	var offered []domain.ClientSelection
	for _, provider := range providers {
		cfg.Providers[provider] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://" + provider + ".test"}}
		ref := domain.NewModelRef(provider, "fable")
		cfg.Models[ref] = domain.Model{Provider: provider, Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
		offered = append(offered, domain.ClientSelection{Model: ref})
	}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{Primary: offered[0], Alternatives: offered[1:]}
	return cfg
}

// With no client enabled, the next step is what gives a recommended provider
// a Token. `aigw provider rotate` would ask for a Token, send it to the
// provider and then meet a Keychain item that macOS refuses AIGW, so such a
// provider contributes the fix of its item, never the bare command: alone
// when it is the one compatible provider, and after the commands of the
// providers without a Token as a sentence of its own, which names the
// provider by the refusal's title.
func TestARefusedAlternativeNamesTheFixOfItsItem(t *testing.T) {
	gateway, gatewayRefusal := refusedSlot("gateway")
	other, otherRefusal := refusedSlot("other")
	sentence := func(refusal *secret.KeychainItemRefusedError) string { return refusal.Title() + ": " + refusal.Fix() }
	for _, test := range []struct {
		name      string
		providers []string
		slots     map[string]secret.TokenObservation
		want      string
	}{
		{name: "one refused provider", providers: []string{"gateway"}, slots: map[string]secret.TokenObservation{"gateway": gateway},
			want: gatewayRefusal.Fix()},
		{name: "a refused provider and one without a Token", providers: []string{"gateway", "spare"},
			slots: map[string]secret.TokenObservation{"gateway": gateway},
			want:  "Choose one compatible provider: aigw provider rotate spare. " + sentence(gatewayRefusal)},
		{name: "a refused provider and two without a Token", providers: []string{"team", "gateway", "spare"},
			slots: map[string]secret.TokenObservation{"gateway": gateway},
			want:  "Choose one compatible provider: aigw provider rotate spare or aigw provider rotate team. " + sentence(gatewayRefusal)},
		{name: "two refused providers", providers: []string{"other", "gateway"},
			slots: map[string]secret.TokenObservation{"gateway": gateway, "other": other},
			want:  sentence(gatewayRefusal) + " " + sentence(otherRefusal)},
		{name: "a refused provider and an empty Token", providers: []string{"gateway", "spare"},
			slots: map[string]secret.TokenObservation{"gateway": gateway, "spare": {Present: true, Empty: true}},
			want:  "Choose one compatible provider: aigw provider rotate spare. " + sentence(gatewayRefusal)},
	} {
		activation := AssessActivation(recommending(test.providers...), observedSlots{slots: test.slots})
		if got := activation.NextActionFor(nil); got != test.want {
			t.Errorf("%s: the next step is\n%q\nwant\n%q", test.name, got, test.want)
		}
		if got := activation.ClientCredentialPrerequisites[claude.ID]; got == "" {
			t.Errorf("%s: Claude Code has no step of its own", test.name)
		}
	}
}
