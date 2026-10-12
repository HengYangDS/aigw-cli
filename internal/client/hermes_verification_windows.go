//go:build windows

package client

import (
	"context"
	"errors"

	"aigw-cli/internal/configuration"
)

// verifyIsolated refuses before starting the client: installed Hermes republishes
// its launchers from any home on Windows and ignores the setting that disables it.
func verifyIsolated(context.Context, Dependencies, configuration.ClientBinding, []byte) (Verification, error) {
	return Verification{}, errors.New("Hermes verification cannot isolate native launcher publication on Windows; no client process was started")
}
