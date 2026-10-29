package domain

import (
	"reflect"
	"strings"
	"testing"
)

// testAdmission admits the fixture built-in clients for this package's tests.
func testAdmission() Admission { return NewAdmission(testClientSpecs()...) }

func TestAdmissionKeepsOrderAndReturnsCopies(t *testing.T) {
	input := ClientSpec{
		ID: "second", Label: "Second", EndpointProtocols: []EndpointProtocol{ProtocolAnthropic},
		QualifiedModes: []string{"Chat"}, QualifiedPlatforms: []string{"macOS"},
	}
	admission := NewAdmission(input, ClientSpec{ID: "first", Label: "First", ProductName: "First Product"})
	if got := admission.IDs(); !reflect.DeepEqual(got, []string{"second", "first"}) {
		t.Fatalf("IDs() = %v", got)
	}
	mutate := func(spec ClientSpec) {
		spec.EndpointProtocols[0], spec.QualifiedModes[0], spec.QualifiedPlatforms[0] = ProtocolOpenAIResponses, "Code", "Linux"
	}
	unshared := func(source string) {
		t.Helper()
		again, _ := admission.Spec("second")
		if again.EndpointProtocols[0] != ProtocolAnthropic || again.QualifiedModes[0] != "Chat" || again.QualifiedPlatforms[0] != "macOS" {
			t.Fatalf("%s shares its slices with the admission: %#v", source, again)
		}
	}
	mutate(input)
	unshared("the admitted argument")
	spec, admitted := admission.Spec("second")
	if !admitted || spec.Label != "Second" {
		t.Fatalf("Spec(second) = %#v, %v", spec, admitted)
	}
	mutate(spec)
	unshared("Spec")
	mutate(admission.Specs()[0])
	unshared("Specs")
	if admission.Admits("third") {
		t.Fatal("an unknown client is admitted")
	}
	if got := admission.Usage("all"); got != "second, first, or all" {
		t.Fatalf("Usage(all) = %q", got)
	}
	if admission.Label("first") != "First" || admission.Label("third") != "third" {
		t.Fatalf("Label = %q, %q", admission.Label("first"), admission.Label("third"))
	}
	if admission.ProductName("first") != "First Product" || admission.ProductName("second") != "Second" ||
		admission.ProductName("third") != "third" {
		t.Fatal("ProductName does not fall back to the label and then the identifier")
	}
}

func TestAdmissionRefusesAnInvalidSpecification(t *testing.T) {
	for name, specs := range map[string][]ClientSpec{
		"empty identifier":    {{ID: ""}},
		"repeated identifier": {{ID: "one"}, {ID: "one"}},
		// Client-native authentication requires an explicit model provider,
		// which only a client with a default model provider accepts.
		"native authentication without a model provider": {{ID: "native", ClientNativeAuthentication: true}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("NewAdmission accepted an invalid set")
				}
			}()
			NewAdmission(specs...)
		})
	}
}

func TestConfigCarriesItsAdmissionThroughClone(t *testing.T) {
	cfg := NewConfig(NewAdmission(ClientSpec{ID: "only"}))
	clone := cfg.Clone()
	if got := clone.Admission().IDs(); !reflect.DeepEqual(got, []string{"only"}) {
		t.Fatalf("the clone admits %v", got)
	}
}

// TestClientRulesFollowTheAdmittedCapabilities admits fixture clients so that
// each client-specific rule and each error that names the capable clients is
// shown to come from the specifications, not from a built-in client name.
func TestClientRulesFollowTheAdmittedCapabilities(t *testing.T) {
	responses := []EndpointProtocol{ProtocolOpenAIResponses}
	plain := ClientSpec{ID: "plain", Label: "Plain", EndpointProtocols: responses}
	provider := ClientSpec{ID: "provider", Label: "Provider", EndpointProtocols: responses, DefaultModelProvider: "provider-default"}
	native := ClientSpec{
		ID: "native", Label: "Native", EndpointProtocols: responses, DefaultModelProvider: "native-default",
		ClientNativeAuthentication: true, RetryBudgets: true, PublishedProfileProtocol: ProtocolOpenAIResponses,
	}
	other := native
	other.ID, other.Label = "other", "Other"
	admission := NewAdmission(plain, provider, native, other)
	base := NewConfig(admission)
	base.Accounts["gateway"] = Account{Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	base.Routes["route"] = testRoute("Route", "gateway", "model", ProtocolOpenAIResponses)

	for name, test := range map[string]struct {
		client  string
		binding ClientBinding
		want    string
	}{
		"model provider": {
			client: "plain", binding: ClientBinding{Route: "route", ModelProvider: "explicit"},
			want: "model_provider is only supported for provider, native, or other",
		},
		"client-native authentication": {
			client: "provider", binding: ClientBinding{Route: "route", ModelProvider: "explicit", Authentication: AuthenticationClientNative},
			want: "client-native authentication is only supported for native or other",
		},
		"retry budgets": {
			client: "provider", binding: ClientBinding{Route: "route", RetryPolicy: &RetryPolicy{RequestMaxRetries: new(uint64)}},
			want: `client "provider" has no admitted native retry-budget capability`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := base.Clone()
			cfg.Clients[test.client] = test.binding
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() = %v, want %q", err, test.want)
			}
		})
	}

	cfg := base.Clone()
	cfg.Clients["plain"] = ClientBinding{Route: "route"}
	cfg.Clients["native"] = ClientBinding{Route: "route", RetryPolicy: &RetryPolicy{RequestMaxRetries: new(uint64)}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("capable clients are refused: %v", err)
	}
	for client, want := range map[string]string{"plain": "", "native": "native-default"} {
		if runtime, err := cfg.ResolveRuntime(client, ""); err != nil || runtime.ModelProvider != want {
			t.Fatalf("%s model provider = %q, %v; want %q", client, runtime.ModelProvider, err, want)
		}
	}
}
