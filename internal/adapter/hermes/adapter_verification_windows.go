//go:build windows

package hermes

import (
	"context"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// verifyIsolated consumes the platform home factory's refusal before any probe.
func verifyIsolated(_ context.Context, _ adapter.Dependencies, configured domain.ClientBinding, _ []byte) (adapter.Verification, error) {
	_, _, err := CreateVerificationHome(configured.Targets[0])
	return adapter.Verification{}, err
}
