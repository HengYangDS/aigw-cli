//go:build windows

package hermes

import (
	"context"

	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
)

// verifyIsolated consumes the platform home factory's refusal before any probe.
func verifyIsolated(_ context.Context, _ client.Dependencies, configured configuration.ClientBinding, _ []byte) (client.Verification, error) {
	_, _, err := CreateVerificationHome(configured.Targets[0])
	return client.Verification{}, err
}
