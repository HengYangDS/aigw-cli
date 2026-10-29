package configuration

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testAdmission admits the fixture built-in clients for this package's tests.
func testAdmission() Admission { return NewAdmission(testClientSpecs()...) }

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

	published := publishedConfig{
		Accounts: map[string]Account{"gateway": base.Accounts["gateway"]},
		Profiles: map[string]publishedProfile{"route": {Label: "Route", Account: "gateway", Client: "native", Model: "model"}},
	}
	migrated, err := migratePublishedConfig(published, admission)
	if err != nil {
		t.Fatal(err)
	}
	if protocols := migrated.Routes["route"].AdmittedProtocols(); !reflect.DeepEqual(protocols, responses) {
		t.Fatalf("migrated Route protocols = %v", protocols)
	}
	published.Profiles["route"] = publishedProfile{Label: "Route", Account: "gateway", Client: "provider", Model: "model"}
	if _, err := migratePublishedConfig(published, admission); err == nil || !strings.Contains(err.Error(), `no admitted client protocol for "provider"`) {
		t.Fatalf("a profile of a client without a published protocol migrated: %v", err)
	}
}

// TestStoreAndManifestFollowTheInjectedAdmission shows that loading, saving,
// checkpoints and manifests read the admission that the store and the
// configuration carry: a client that only it admits round-trips, and the same
// bytes refuse that client under an admission without it.
func TestStoreAndManifestFollowTheInjectedAdmission(t *testing.T) {
	future := ClientSpec{ID: "future", Label: "Future", EndpointProtocols: []EndpointProtocol{ProtocolOpenAIResponses}}
	admission := NewAdmission(future)
	cfg := NewConfig(admission)
	cfg.Accounts["gateway"] = Account{Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["route"] = testRoute("Route", "gateway", "model", ProtocolOpenAIResponses)
	cfg.Recommendations["future"] = ClientRecommendation{Primary: ClientSelection{Route: "route"}}
	cfg.SetSelectedRoute("future", "route", "")
	cfg.SetClientActivation("future", true, "/opt/future", nil)
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path, admission)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || !reflect.DeepEqual(loaded.EnabledClientIDs(), []string{"future"}) {
		t.Fatalf("Load() enabled %v, %v", loaded.EnabledClientIDs(), err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), loaded, []string{"future"}); err != nil {
		t.Fatal(err)
	}
	if checkpoint, err := store.LoadVerifiedCheckpoint(); err != nil || checkpoint.Config.SelectedRoute("future") != "route" {
		t.Fatalf("LoadVerifiedCheckpoint() = %#v, %v", checkpoint, err)
	}
	manifest, err := Export(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(manifest, admission); err != nil {
		t.Fatalf("Parse() refused its own export: %v", err)
	}

	if _, err := NewStore(path, testAdmission()).Load(); err == nil || !strings.Contains(err.Error(), `"future"`) {
		t.Fatalf("an admission without the client loaded its binding: %v", err)
	}
	if _, err := Parse(manifest, testAdmission()); err == nil || !strings.Contains(err.Error(), `"future"`) {
		t.Fatalf("an admission without the client parsed its recommendation: %v", err)
	}
	cfg.Clients[ClientCodex] = ClientBinding{Route: "route"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), `unknown client binding "codex"`) {
		t.Fatalf("a client outside the admission was accepted: %v", err)
	}
}
