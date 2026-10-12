//go:build windows

package client

import (
	"context"

	"aigw-cli/internal/configuration"
	hermesconfig "aigw-cli/internal/hermes/configuration"
)

func (hermesAdapter) Verify(_ context.Context, _ Dependencies, _ configuration.Config, _ configuration.Runtime, _ string) (Verification, error) {
	return Verification{}, hermesconfig.ErrNativeVerificationUnsupported
}
