package synchronization

import (
	"context"
	"errors"
	"fmt"

	"aigw-cli/internal/client"
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
func (s Synchronizer) CommitProjection(ctx context.Context, before, after configuration.Config, subject string, clientIDs ...string) error {
	return s.commit(ctx, before, after, subject, true, clientIDs...)
}

func (s Synchronizer) commit(ctx context.Context, before, after configuration.Config, subject string, reconcileProjection bool, clientIDs ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	configBefore, err := s.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	var projectable []string
	if reconcileProjection {
		projectable, err = s.credentialReadyClients(after, clientIDs...)
		if err != nil {
			return fmt.Errorf("%s synchronization preflight failed; configuration and client files were unchanged: %w", subject, err)
		}
	}
	if len(projectable) > 0 {
		if _, err := s.registry().Plan(s.clientDependencies(), before, after, projectable...); err != nil {
			return fmt.Errorf("%s synchronization preflight failed; configuration and client files were unchanged: %w", subject, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var undoEntrypoint func() error
	if reconcileProjection {
		undoEntrypoint, err = s.prepareCredentialEntrypoint(after, clientIDs...)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
	}
	configAfter, err := s.Config.Commit(configBefore, after)
	if err != nil {
		return errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
	}
	if len(projectable) > 0 {
		if err := s.applyProjection(ctx, before, after, configBefore, configAfter, undoEntrypoint, projectable...); err != nil {
			return fmt.Errorf("%s %w", subject, err)
		}
	}
	if reconcileProjection {
		if err := s.finalizeCredentialEntrypoint(after); err != nil {
			return fmt.Errorf("%s configuration and client projections completed, but credential entrypoint finalization failed: %w", subject, err)
		}
	}
	return nil
}

func (s Synchronizer) finalizeCredentialEntrypoint(cfg configuration.Config) error {
	action, err := s.CredentialEntrypointPlan(cfg)
	if err != nil {
		return fmt.Errorf("inspect credential entrypoint: %w", err)
	}
	switch action {
	case CredentialEntrypointInstall:
		return errors.New("credential entrypoint disappeared after client projection")
	case CredentialEntrypointUnchanged:
		return nil
	default:
		return fmt.Errorf("unsupported credential entrypoint action %q", action)
	}
}

func (s Synchronizer) applyProjection(
	ctx context.Context,
	before, after configuration.Config,
	configBefore, configAfter configuration.Snapshot,
	undoEntrypoint func() error,
	clientIDs ...string,
) error {
	if err := s.registry().Apply(ctx, s.clientDependencies(), before, after, clientIDs...); err != nil {
		if rollbackErr := s.Config.RestoreSnapshot(configBefore, configAfter); rollbackErr != nil {
			return fmt.Errorf("synchronization failed: %w; rollback also failed: %w", err, rollbackErr)
		}
		if !errors.Is(err, client.ErrProjectionRollbackFailed) {
			err = errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
		}
		return fmt.Errorf("synchronization failed; configuration was rolled back: %w", err)
	}
	return nil
}

func undoCreatedEntrypoint(undo func() error) error {
	if undo == nil {
		return nil
	}
	return undo()
}
