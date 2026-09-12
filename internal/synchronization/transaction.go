package synchronization

import (
	"context"
	"fmt"

	configuration "aigw-cli/internal/configuration"
)

// Commit persists one configuration transition and converges affected client
// projections. Failures compensate owned writes without changing native credentials.
func (s Synchronizer) Commit(ctx context.Context, before, after configuration.Config, subject string) error {
	clients := s.registry().ChangedClients(before, after)
	return s.commit(ctx, before, after, subject, len(clients) > 0, clients...)
}

// CommitProjection persists configuration and reconciles every client projection,
// including missing or out-of-date projections of unchanged configuration.
func (s Synchronizer) CommitProjection(ctx context.Context, before, after configuration.Config, subject string) error {
	return s.commit(ctx, before, after, subject, true)
}

func (s Synchronizer) commit(ctx context.Context, before, after configuration.Config, subject string, reconcileProjection bool, clientIDs ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	configBefore, err := s.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	if reconcileProjection {
		if _, err := s.registry().Plan(s.clientDependencies(), before, after, clientIDs...); err != nil {
			return fmt.Errorf("%s synchronization preflight failed; configuration and client files were unchanged: %w", subject, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	configAfter, err := s.Config.Commit(configBefore, after)
	if err != nil {
		return err
	}
	if reconcileProjection {
		if err := s.registry().Apply(ctx, s.clientDependencies(), before, after, clientIDs...); err != nil {
			if rollbackErr := s.Config.RestoreSnapshot(configBefore, configAfter); rollbackErr != nil {
				return fmt.Errorf("%s synchronization failed: %w; rollback also failed: %w", subject, err, rollbackErr)
			}
			return fmt.Errorf("%s synchronization failed; configuration was rolled back: %w", subject, err)
		}
	}
	return nil
}
