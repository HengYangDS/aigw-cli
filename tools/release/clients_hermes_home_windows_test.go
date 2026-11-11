//go:build client_acceptance && windows

package main

import (
	"path/filepath"

	"aigw-cli/internal/adapter/hermes"
)

// isolateHermesHome refuses on Windows, where Hermes verification cannot
// isolate its native launcher, before any client process starts.
func (j *journeyFixture) isolateHermesHome(string) error {
	installation, err := requiredClientInput("AIGW_ACCEPTANCE_HERMES_HOME", true)
	if err != nil {
		return err
	}
	_, _, err = hermes.CreateVerificationHome(filepath.Join(installation, "config.yaml"))
	return err
}
