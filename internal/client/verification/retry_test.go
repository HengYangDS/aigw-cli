package verification

import (
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestVerificationCommandKeepsOnlyAnExplicitRoute(t *testing.T) {
	if got := Command(configuration.ClientHermes, ""); got != "aigw verify --for hermes" {
		t.Fatalf("selected-binding command = %q", got)
	}
	if got := Command(configuration.ClientCodex, "team.codex-1"); got != "aigw verify --for codex --route team.codex-1" {
		t.Fatalf("explicit-Route command = %q", got)
	}
}
