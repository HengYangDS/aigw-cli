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
	if len(projectable) > 0 {
		undoEntrypoint, err = s.prepareCredentialEntrypoint(after, projectable...)
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
	if len(projectable) == 0 {
		return nil
	}
	receipt, err := s.applyProjection(ctx, before, after, configBefore, configAfter, undoEntrypoint, projectable...)
	if err != nil {
		return fmt.Errorf("%s %w", subject, err)
	}
	if err := s.finalizeCredentialEntrypoint(after, projectable...); err != nil {
		projectionErr := receipt.Rollback()
		configErr := s.Config.RestoreSnapshot(configBefore, configAfter)
		if projectionErr != nil {
			projectionErr = fmt.Errorf("%w: %w", client.ErrProjectionRollbackFailed, projectionErr)
		}
		var entrypointErr error
		if projectionErr == nil && configErr == nil {
			entrypointErr = undoCreatedEntrypoint(undoEntrypoint)
		}
		if rollbackErr := errors.Join(projectionErr, configErr, entrypointErr); rollbackErr != nil {
			return fmt.Errorf("%s credential entrypoint finalization failed: %w; compensation incomplete: %w", subject, err, rollbackErr)
		}
		return fmt.Errorf("%s credential entrypoint finalization failed; configuration and client projections were rolled back: %w", subject, err)
	}
	return nil
}

func (s Synchronizer) finalizeCredentialEntrypoint(cfg configuration.Config, clientIDs ...string) error {
	action, err := s.CredentialEntrypointPlan(cfg, clientIDs...)
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
) (client.ProjectionReceipt, error) {
	receipt, err := s.registry().Apply(ctx, s.clientDependencies(), before, after, clientIDs...)
	if err != nil {
		if rollbackErr := s.Config.RestoreSnapshot(configBefore, configAfter); rollbackErr != nil {
			return nil, projectionError{cause: fmt.Errorf("synchronization failed: %w; rollback also failed: %w", err, rollbackErr)}
		}
		if !errors.Is(err, client.ErrProjectionRollbackFailed) {
			err = errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
		}
		return nil, projectionError{cause: fmt.Errorf("synchronization failed; configuration was rolled back: %w", err), restored: true}
	}
	return receipt, nil
}

// projectionError preserves the verified configuration outcome without
// claiming that every client or credential entrypoint was restored.
type projectionError struct {
	cause    error
	restored bool
}

func (e projectionError) Error() string               { return e.cause.Error() }
func (e projectionError) Unwrap() error               { return e.cause }
func (e projectionError) ConfigurationRestored() bool { return e.restored }

func undoCreatedEntrypoint(undo func() error) error {
	if undo == nil {
		return nil
	}
	return undo()
}
