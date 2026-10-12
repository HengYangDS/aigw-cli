//go:build windows

package client

import (
	"context"

	"aigw-cli/internal/configuration"
	hermesconfig "aigw-cli/internal/hermes/configuration"
)

// verifyIsolated consumes the platform home factory's refusal before any probe.
func verifyIsolated(_ context.Context, _ Dependencies, configured configuration.ClientBinding, _ []byte) (Verification, error) {
	_, _, err := hermesconfig.CreateVerificationHome(configured.Targets[0])
	return Verification{}, err
}
