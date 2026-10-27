package selection

import (
	"strings"
	"testing"
)

// --direct with --forwarding-endpoint conflicts even when the endpoint is empty;
// advising --direct, which the operator already gave, would not resolve it. An
// empty endpoint alone is refused with the advice to use --direct.
func TestUseNamesTheDirectConflictForAnEmptyForwardingEndpoint(t *testing.T) {
	if _, err := forwardingChoice("", true, true); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("--direct with an empty --forwarding-endpoint = %v", err)
	}
	if _, err := forwardingChoice("", true, false); err == nil || !strings.Contains(err.Error(), "use --direct to remove forwarding") {
		t.Fatalf("an empty --forwarding-endpoint = %v", err)
	}
}
