package rollback

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// verifiedRecoveryError reports a configuration outcome as a failed commit does.
type verifiedRecoveryError struct{ restored bool }

func (verifiedRecoveryError) Error() string                        { return "client projection failed" }
func (recovery verifiedRecoveryError) ConfigurationRestored() bool { return recovery.restored }

func TestRollbackFailureReportsVerifiedConfigurationRecovery(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "restored", err: fmt.Errorf("rollback %w", verifiedRecoveryError{restored: true}), want: "Configuration was restored"},
		{name: "incomplete", err: verifiedRecoveryError{}, want: "Configuration restoration was incomplete"},
		{name: "unverified", err: errors.New("rollback synchronization preflight failed"), want: "may have changed; inspect current state"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rollbackFailureImpact(test.err); !strings.Contains(got, test.want) {
				t.Fatalf("rollback failure impact = %q, want %q", got, test.want)
			}
		})
	}
}
