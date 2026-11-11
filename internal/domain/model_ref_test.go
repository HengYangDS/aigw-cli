package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestParseModelRefSplitsAtTheFirstSlash holds the model argument's grammar:
// the provider is the text before the first slash and the rest is the
// provider's own model ID, which may contain a slash; an argument without a
// slash, an empty part and an invalid provider are refused.
func TestParseModelRefSplitsAtTheFirstSlash(t *testing.T) {
	for text, want := range map[string][2]string{
		"aihubmix/openai/gpt-oss": {"aihubmix", "openai/gpt-oss"},
		"dmxapi/gpt-6.1-sol":      {"dmxapi", "gpt-6.1-sol"},
		"ucloud/MiniMax-M3":       {"ucloud", "MiniMax-M3"},
		"team/a b":                {"team", "a b"},
	} {
		ref, err := ParseModelRef(text)
		if err != nil || ref.Provider() != want[0] || ref.ID() != want[1] || ref.String() != text {
			t.Fatalf("ParseModelRef(%q) = %q %q %q, %v; want %q", text, ref.Provider(), ref.ID(), ref.String(), err, want)
		}
	}
	if _, err := ParseModelRef("gpt-6.1-sol"); !errors.Is(err, ErrModelRefWithoutProvider) || !strings.Contains(err.Error(), `"gpt-6.1-sol" has no provider`) {
		t.Fatalf("an argument without a slash = %v", err)
	}
	for _, text := range []string{"/gpt", "dmxapi/", "/"} {
		if _, err := ParseModelRef(text); err == nil || errors.Is(err, ErrModelRefWithoutProvider) || !strings.Contains(err.Error(), "empty provider or model ID") {
			t.Fatalf("ParseModelRef(%q) = %v, want an empty-part refusal", text, err)
		}
	}
	for _, text := range []string{"bad provider/model", "-team/model", "té/model"} {
		if _, err := ParseModelRef(text); err == nil || !strings.Contains(err.Error(), "invalid provider") {
			t.Fatalf("ParseModelRef(%q) = %v, want an invalid-provider refusal", text, err)
		}
	}
}

// TestResolveModelRefFindsTheProvidersModel resolves a reference on today's
// configuration: the one model of its provider whose provider model ID equals
// the reference's, which differs from its key; a case-only difference, another
// provider's model and an unknown ID resolve to nothing.
func TestResolveModelRefFindsTheProvidersModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["ucloud"] = Provider{Label: "UCloud", Endpoints: Endpoints{Responses: "https://ucloud.test/v1"}}
	cfg.Providers["dmxapi"] = Provider{Label: "DMXAPI", Endpoints: Endpoints{Responses: "https://dmxapi.test/v1"}}
	responses := map[EndpointProtocol][]Capability{ProtocolResponses: {}}
	cfg.Models["ucloud-minimax"] = Model{Provider: "ucloud", Family: "minimax-m3", UpstreamModel: "MiniMax-M3", Protocols: responses}
	cfg.Models["dmxapi-sol"] = Model{Provider: "dmxapi", Family: "gpt-6.1-sol", Protocols: responses}
	cfg.Models["ucloud-sol"] = Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: responses}
	for text, want := range map[string]string{"ucloud/MiniMax-M3": "ucloud-minimax", "dmxapi/gpt-6.1-sol": "dmxapi-sol", "ucloud/gpt-6.1-sol": "ucloud-sol"} {
		ref, _ := ParseModelRef(text)
		if keys := cfg.ResolveModelRef(ref); !slices.Equal(keys, []string{want}) {
			t.Fatalf("ResolveModelRef(%s) = %q; want %q", text, keys, want)
		}
		if got := cfg.Models[want].Ref(); got != ref {
			t.Fatalf("%s.Ref() = %s, want %s", want, got, ref)
		}
	}
	for _, text := range []string{"ucloud/minimax-m3", "dmxapi/MiniMax-M3", "ucloud/unknown", "other/gpt-6.1-sol"} {
		ref, _ := ParseModelRef(text)
		if keys := cfg.ResolveModelRef(ref); len(keys) != 0 {
			t.Fatalf("ResolveModelRef(%s) = %q, want no model", text, keys)
		}
	}
	// Today's configuration can give one name a Route per protocol; the
	// lookup returns each, in key order.
	cfg.Providers["ucloud"] = Provider{Label: "UCloud", Endpoints: Endpoints{Responses: "https://ucloud.test/v1", Chat: "https://ucloud.test/v1"}}
	cfg.Models["ucloud-sol-chat"] = Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: map[EndpointProtocol][]Capability{ProtocolChat: {}}}
	ref, _ := ParseModelRef("ucloud/gpt-6.1-sol")
	if keys := cfg.ResolveModelRef(ref); !slices.Equal(keys, []string{"ucloud-sol", "ucloud-sol-chat"}) {
		t.Fatalf("ResolveModelRef of a name with two Routes = %q", keys)
	}
}
