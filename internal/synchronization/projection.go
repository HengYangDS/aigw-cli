package synchronization

import (
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
)

// Plan returns every side-effect-free client projection change for a
// configuration transition, including target removals.
func (s Synchronizer) Plan(before, after configuration.Config) ([]client.ProjectionPlan, error) {
	return s.registry().Plan(s.clientDependencies(), before, after)
}
