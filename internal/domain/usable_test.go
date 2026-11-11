package domain

import (
	"strings"
	"testing"
)

// A model without a protocol, absent or empty, is refused by name: no client
// speaks no protocol, so no client could use it.
func TestAModelWithoutAProtocolIsRefused(t *testing.T) {
	for name, protocols := range map[string]map[EndpointProtocol][]Capability{
		"absent": nil,
		"empty":  {},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			ref := NewModelRef("dmx", "bare")
			cfg.Models[ref] = Model{Provider: "dmx", ID: "bare", Family: "bare", Protocols: protocols}
			cfg.Normalize()
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "model dmx/bare declares no protocol, so no client can use it") {
				t.Fatalf("error = %v", err)
			}
			if entry, named := EntryOf(err); !named || entry != "model `dmx/bare`" {
				t.Fatalf("the refusal names the entry %q, %v", entry, named)
			}
		})
	}
}

// Two provider IDs that differ only in case share one Token slot on a
// case-insensitive store, so validation and a team file refuse the second.
// A capitalized ID alone collides with nothing: it is refused as one that
// must be lowercase, naming its lowercase form, and no collision.
func TestProviderIDsThatDifferOnlyInCaseAreRefused(t *testing.T) {
	cfg := validConfig()
	cfg.Providers["DMX"] = Provider{Label: "Upper", Endpoints: Endpoints{Responses: "https://upper.test/v1"}}
	cfg.Normalize()
	err := cfg.Validate()
	const reason = "one Token slot on a case-insensitive store"
	if err == nil || !strings.Contains(err.Error(), `provider "dmx" differs only in case from provider "DMX"`) || !strings.Contains(err.Error(), reason) {
		t.Fatalf("a pair that differs in case = %v", err)
	}
	cfg = validConfig()
	cfg.Providers["Backup2"] = Provider{Label: "Upper", Endpoints: Endpoints{Responses: "https://upper.test/v1"}}
	cfg.Normalize()
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), `provider "Backup2" must be lowercase; use "backup2"`) ||
		strings.Contains(err.Error(), "differs only in case") || strings.Contains(err.Error(), reason) {
		t.Fatalf("a capitalized provider = %v", err)
	}
	other := NewConfig(testAdmission())
	if err := other.AppendProvider("dmxapi", Provider{Label: "Lower", Endpoints: Endpoints{Responses: "https://lower.test/v1"}}); err != nil {
		t.Fatal(err)
	}
	err = other.AppendProvider("DMXAPI", Provider{Label: "Upper", Endpoints: Endpoints{Responses: "https://upper.test/v1"}})
	if err == nil || !strings.Contains(err.Error(), `provider "DMXAPI" differs only in case from provider "dmxapi"`) || !strings.Contains(err.Error(), reason) {
		t.Fatalf("appending the second provider = %v", err)
	}
}
