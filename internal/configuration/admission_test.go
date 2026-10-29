package configuration

import (
	"reflect"
	"testing"
)

// testAdmission admits the built-in clients for this package's tests.
func testAdmission() Admission { return NewAdmission(AdmittedClientSpecs()...) }

func TestAdmissionKeepsOrderAndReturnsCopies(t *testing.T) {
	admission := NewAdmission(
		ClientSpec{ID: "second", Label: "Second", EndpointProtocols: []EndpointProtocol{ProtocolAnthropic}},
		ClientSpec{ID: "first", Label: "First", ProductName: "First Product"},
	)
	if got := admission.IDs(); !reflect.DeepEqual(got, []string{"second", "first"}) {
		t.Fatalf("IDs() = %v", got)
	}
	spec, admitted := admission.Spec("second")
	if !admitted || spec.Label != "Second" {
		t.Fatalf("Spec(second) = %#v, %v", spec, admitted)
	}
	spec.EndpointProtocols[0] = ProtocolOpenAIResponses
	if again, _ := admission.Spec("second"); again.EndpointProtocols[0] != ProtocolAnthropic {
		t.Fatal("Spec shares its protocols with the admission")
	}
	specs := admission.Specs()
	specs[0].EndpointProtocols[0] = ProtocolOpenAIResponses
	if again, _ := admission.Spec("second"); again.EndpointProtocols[0] != ProtocolAnthropic {
		t.Fatal("Specs shares its protocols with the admission")
	}
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

func TestAdmissionRefusesAnEmptyOrRepeatedIdentifier(t *testing.T) {
	for name, specs := range map[string][]ClientSpec{
		"empty":    {{ID: ""}},
		"repeated": {{ID: "one"}, {ID: "one"}},
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
