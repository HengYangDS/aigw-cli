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
		spec.EndpointProtocols[0], spec.QualifiedModes[0], spec.QualifiedPlatforms[0] = ProtocolResponses, "Code", "Linux"
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

// A specification is refused without an identifier, or with half of an
// extension: an extension ID without the program's path inside it, or that
// path without the ID, which could resolve no program.
func TestAdmissionRefusesAnInvalidSpecification(t *testing.T) {
	for name, extension := range map[string]ClientExtension{
		"an extension without its program": {ID: "publisher.client"},
		"a program without its extension":  {Program: "bin/client"},
	} {
		spec := ClientSpec{ID: "one", Extension: extension}
		if err := spec.Validate(); err == nil || !strings.Contains(err.Error(), `client "one"`) {
			t.Errorf("%s: Validate() = %v", name, err)
		}
	}
	if err := (ClientSpec{ID: "one", Extension: ClientExtension{ID: "publisher.client", Program: "bin/client"}}).Validate(); err != nil {
		t.Errorf("a whole extension is refused: %v", err)
	}
	for name, specs := range map[string][]ClientSpec{
		"empty identifier":    {{ID: ""}},
		"repeated identifier": {{ID: "one"}, {ID: "one"}},
		"half an extension":   {{ID: "one", Extension: ClientExtension{ID: "publisher.client"}}},
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
// a client-specific rule, retry budgets, is shown to come from the
// specifications, not from a built-in client name.
func TestClientRulesFollowTheAdmittedCapabilities(t *testing.T) {
	responses := []EndpointProtocol{ProtocolResponses}
	plain := ClientSpec{ID: "plain", Label: "Plain", EndpointProtocols: responses}
	native := ClientSpec{ID: "native", Label: "Native", EndpointProtocols: responses, RetryBudgets: true}
	admission := NewAdmission(plain, native)
	base := NewConfig(admission)
	base.Providers["gateway"] = Provider{Label: "Gateway", Endpoints: Endpoints{Responses: "https://gateway.test/v1"}}
	base.Models[NewModelRef("gateway", "model")] = testModel("Route", "gateway", "model", ProtocolResponses)

	for name, test := range map[string]struct {
		client  string
		binding ClientBinding
		want    string
	}{
		"retry budgets": {
			client: "plain", binding: ClientBinding{Model: NewModelRef("gateway", "model"), RetryPolicy: &RetryPolicy{RequestMaxRetries: new(uint64)}},
			want: `client "plain" has no admitted native retry-budget capability`,
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
	cfg.Clients["plain"] = ClientBinding{Model: NewModelRef("gateway", "model")}
	cfg.Clients["native"] = ClientBinding{Model: NewModelRef("gateway", "model"), RetryPolicy: &RetryPolicy{RequestMaxRetries: new(uint64)}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("capable clients are refused: %v", err)
	}
}
