// Package claude owns Claude Code route planning and its native integration.
// It never persists provider credentials or modifies user shell profiles.
package claude

import (
	"fmt"

	"aigw-cli/internal/discovery"
)

// Ready reports whether executable resolves to a runnable native client.
func Ready(executable string) (bool, error) {
	ready, err := discovery.ExecutableAvailable(executable)
	if err != nil {
		return false, fmt.Errorf("inspect Claude executable: %w", err)
	}
	return ready, nil
}
