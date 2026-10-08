package synchronization

import (
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
)

// Plan returns side-effect-free changes for credential-ready clients,
// including target removals. Known-missing Tokens leave projections untouched.
func (s Synchronizer) Plan(before, after configuration.Config, clients ...string) ([]client.ProjectionPlan, error) {
	projectable, err := s.credentialReadyClients(after, clients...)
	if err != nil || len(projectable) == 0 {
		return nil, err
	}
	dependencies, err := s.clientDependencies(projectable, before, after)
	if err != nil {
		return nil, err
	}
	return s.registry().Plan(dependencies, before, after, projectable...)
}
