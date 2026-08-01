package synchronization

import (
	"context"
	"fmt"

	configuration "aigw-cli/internal/configuration"
)

// Commit persists one configuration transition and converges every affected
// Codex projection and native authentication target. Any downstream failure
// restores both configuration and projections to their verified preimages.
func (s Synchronizer) Commit(ctx context.Context, before, after configuration.Config, subject string) error {
	configBefore, err := s.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	if err := s.Config.Save(after); err != nil {
		return err
	}
	configAfter, err := s.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	if ProjectionChanged(before, after) {
		if err := s.Reconcile(ctx, before, after); err != nil {
			rollbackErr := s.rollback(ctx, before, after, configBefore, configAfter, false)
			if rollbackErr != nil {
				return fmt.Errorf("%s synchronization failed: %w; rollback also failed: %v", subject, err, rollbackErr)
			}
			return fmt.Errorf("%s synchronization failed and was rolled back: %w", subject, err)
		}
	}
	if AuthenticationChanged(before, after) {
		if err := s.BindAuthentication(ctx, after); err != nil {
			rollbackErr := s.rollback(ctx, before, after, configBefore, configAfter, true)
			if rollbackErr != nil {
				return fmt.Errorf("%s authentication failed: %w; rollback also failed: %v", subject, err, rollbackErr)
			}
			return fmt.Errorf("%s authentication failed and was rolled back: %w", subject, err)
		}
	}
	return nil
}

func (s Synchronizer) rollback(ctx context.Context, before, after configuration.Config, configBefore, configAfter configuration.Snapshot, rebindNativeAuthentication bool) error {
	if err := s.Config.RestoreSnapshot(configBefore, configAfter); err != nil {
		return err
	}
	if err := s.Reconcile(ctx, after, before); err != nil {
		return err
	}
	if rebindNativeAuthentication {
		return s.BindAuthentication(ctx, before)
	}
	return nil
}
