package onboarding

import (
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestManifestSetupAvailabilityFollowsTheAdmittedClientRegistry(t *testing.T) {
	executables := map[string]string{}
	for _, clientID := range configuration.AdmittedClientIDs() {
		executables[clientID] = "/installed/" + clientID
	}
	available := manifestSetupAvailableClients(executables)
	for _, clientID := range configuration.AdmittedClientIDs() {
		if !available[clientID] {
			t.Fatalf("admitted installed client %q is unavailable: %#v", clientID, available)
		}
	}
}
