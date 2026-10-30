package verification

import (
	"testing"
)

func TestVerificationCommandKeepsOnlyAnExplicitRoute(t *testing.T) {
	if got := Command(clientHermes, ""); got != "aigw verify --for hermes" {
		t.Fatalf("selected-binding command = %q", got)
	}
	if got := Command(clientCodex, "team.codex-1"); got != "aigw verify --for codex --route team.codex-1" {
		t.Fatalf("explicit-Route command = %q", got)
	}
}
