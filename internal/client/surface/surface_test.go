package surface

import "testing"

func TestStableIdentities(t *testing.T) {
	if got := string(AuthorityAIGW); got != "aigw" {
		t.Fatalf("AIGW authority = %q, want %q", got, "aigw")
	}
}
