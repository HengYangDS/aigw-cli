package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProtocolTimeoutAllowsColdClientStartup(t *testing.T) {
	if ProtocolTimeout < time.Minute {
		t.Fatalf("ProtocolTimeout = %s, want at least %s", ProtocolTimeout, time.Minute)
	}
}

// A client's advice explains its own diagnostics; another client's failure
// with the same diagnostic gets only the shared summary, and a timeout still
// wins over the advice.
func TestFailureAppliesOnlyTheCallersAdvice(t *testing.T) {
	cause := errors.New("launch /Users/operator/private/client: exit status 1")
	diagnostic := []byte(`{"result":"API Error: 400 Tier_Gate: extra inputs are not permitted; token=must-not-leak; /Users/operator/private"}`)
	advice := []Advice{{Markers: []string{"tier_gate", "extra inputs are not permitted"}, Summary: "endpoint rejects the tier gate; disable it and verify again"}}
	if err := Failure("Other", "other", "", diagnostic, cause, nil); strings.Contains(err.Error(), "tier gate") {
		t.Fatalf("a failure without advice applied another client's advice: %v", err)
	}
	err := Failure("Client", "client", "", diagnostic, cause, advice, "must-not-leak")
	if !strings.Contains(err.Error(), "Client minimal verification request failed: endpoint rejects the tier gate; disable it and verify again; run `aigw verify --for client`") || !errors.Is(err, cause) {
		t.Fatalf("the caller's advice or cause was lost: %v", err)
	}
	if err := Failure("Client", "client", "", diagnostic, context.DeadlineExceeded, advice); !strings.Contains(err.Error(), "client verification timed out") {
		t.Fatalf("advice outranked a timeout: %v", err)
	}
}
