package configuration

import (
	"strings"
	"testing"
)

func TestRouteLifecycleSeparatesDeprecationFromBindingValidity(t *testing.T) {
	cfg := validConfig()
	route := cfg.Routes["backup"]
	if route.LifecycleState() != RouteAdmitted {
		t.Fatalf("default lifecycle = %q", route.LifecycleState())
	}
	route.Lifecycle = RouteDeprecated
	cfg.Routes["backup"] = route
	if err := cfg.Validate(); err != nil {
		t.Fatalf("an explicit binding to a deprecated Route must remain valid: %v", err)
	}

	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Route: "backup"}}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "deprecated route") {
		t.Fatalf("deprecated recommendation error = %v", err)
	}

	delete(cfg.Recommendations, ClientCodex)
	route.Lifecycle = "retired"
	cfg.Routes["backup"] = route
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "invalid lifecycle") {
		t.Fatalf("invalid lifecycle error = %v", err)
	}
}
