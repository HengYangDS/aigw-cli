package domain

import (
	"strings"
	"testing"
)

func TestModelLifecycleSeparatesDeprecationFromBindingValidity(t *testing.T) {
	cfg := validConfig()
	route := cfg.Models["backup"]
	if route.LifecycleState() != ModelAdmitted {
		t.Fatalf("default lifecycle = %q", route.LifecycleState())
	}
	route.Lifecycle = ModelDeprecated
	cfg.Models["backup"] = route
	if err := cfg.Validate(); err != nil {
		t.Fatalf("an explicit binding to a deprecated Route must remain valid: %v", err)
	}

	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: "backup"}}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "deprecated route") {
		t.Fatalf("deprecated recommendation error = %v", err)
	}

	delete(cfg.Recommendations, ClientCodex)
	route.Lifecycle = "retired"
	cfg.Models["backup"] = route
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "invalid lifecycle") {
		t.Fatalf("invalid lifecycle error = %v", err)
	}
}
