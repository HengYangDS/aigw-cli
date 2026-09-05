package synchronization

import (
	"context"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
)

// Plan returns every side-effect-free client projection change for a
// configuration transition, including target removals.
func (s Synchronizer) Plan(before, after configuration.Config) ([]client.ProjectionPlan, error) {
	return s.registry().Plan(s.clientDependencies(), before, after)
}

// Reconcile applies every admitted client projection through the registry's
// single compensated transaction boundary.
func (s Synchronizer) Reconcile(ctx context.Context, before, after configuration.Config) error {
	_, err := s.registry().Apply(ctx, s.clientDependencies(), before, after)
	return err
}

// ProjectionChanged reports whether any admitted adapter owns a changed
// persistent projection.
func (s Synchronizer) ProjectionChanged(before, after configuration.Config) bool {
	return s.registry().ProjectionChanged(before, after)
}
