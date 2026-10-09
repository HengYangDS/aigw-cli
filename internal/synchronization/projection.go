package synchronization

import (
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
)

// Plan returns side-effect-free changes for the selected clients, including
// target removals. An empty client list selects every admitted adapter.
func (s Synchronizer) Plan(before, after configuration.Config, clientIDs ...string) ([]client.ProjectionPlan, error) {
	return s.registry().Plan(s.clientDependencies(), before, after, clientIDs...)
}
