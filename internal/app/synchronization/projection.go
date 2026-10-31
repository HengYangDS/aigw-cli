package synchronization

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// Plan returns side-effect-free changes for credential-ready clients,
// including target removals. Known-missing Tokens leave projections untouched.
func (s Synchronizer) Plan(before, after domain.Config, clients ...string) ([]adapter.ProjectionPlan, error) {
	projection, err := project(s, before, after, clients...)
	return projection.Targets, err
}
