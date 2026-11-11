package domain

import (
	"slices"
	"testing"
)

// TestProtocolNamesMapToTheProtocols holds the command line's protocol names:
// each valid name maps to its protocol and back, and any other spelling,
// including a protocol's configuration value, names no protocol.
func TestProtocolNamesMapToTheProtocols(t *testing.T) {
	if got := ProtocolNames(); !slices.Equal(got, []string{"anthropic", "responses", "chat"}) {
		t.Fatalf("ProtocolNames() = %q", got)
	}
	if got := ProtocolUsage(); got != "anthropic, responses, or chat" {
		t.Fatalf("ProtocolUsage() = %q", got)
	}
	for name, want := range map[string]EndpointProtocol{
		"anthropic": ProtocolAnthropic,
		"responses": ProtocolResponses,
		"chat":      ProtocolChat,
	} {
		got, named := ProtocolNamed(name)
		if !named || got != want {
			t.Fatalf("ProtocolNamed(%q) = %q, %t; want %q", name, got, named, want)
		}
		if got.Name() != name {
			t.Fatalf("%q.Name() = %q, want %q", got, got.Name(), name)
		}
	}
	for _, other := range []string{"openai_responses", "openai_chat_completions", "Responses", "messages", ""} {
		if got, named := ProtocolNamed(other); named || got != "" {
			t.Fatalf("ProtocolNamed(%q) = %q, %t; want no protocol", other, got, named)
		}
	}
	if got := EndpointProtocol("other").Name(); got != "other" {
		t.Fatalf("an unnamed protocol's Name() = %q", got)
	}
}
