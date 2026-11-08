package verification

import (
	"testing"
)

func TestVerificationCommandKeepsOnlyAnExplicitModel(t *testing.T) {
	if got := Command(clientHermes, ""); got != "aigw check hermes --depth client" {
		t.Fatalf("selected-binding command = %q", got)
	}
	if got := Command(clientCodex, "team/codex-1"); got != "aigw check codex --model team/codex-1 --depth client" {
		t.Fatalf("explicit-Route command = %q", got)
	}
}
