package synchronization

import (
	"context"
	"errors"
	"fmt"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// Commit persists one configuration transition and converges affected client
// projections. Failures compensate owned writes without changing native credentials.
func (s Synchronizer) Commit(ctx context.Context, before, after domain.Config, subject string) error {
	clients := s.registry().ChangedClients(before, after)
	_, err := s.commit(ctx, before, after, subject, len(clients) > 0, clients...)
	return err
}

// CommitProjection persists configuration and reconciles every client projection,
// including missing or out-of-date projections of unchanged configuration.
func (s Synchronizer) CommitProjection(ctx context.Context, before, after domain.Config, subject string, clientIDs ...string) error {
	_, err := s.commit(ctx, before, after, subject, true, clientIDs...)
	return err
}

// CommitRestore persists restored, a configuration that a rollback restores,
// and reconciles every client whose selection the restore decides (see
// restoredClients), so that a restored selection is written or recorded, or
// the restore is refused, and is never silently set aside.
func (s Synchronizer) CommitRestore(ctx context.Context, before, restored domain.Config, subject string) error {
	clients := s.restoredClients(before, restored)
	if len(clients) == 0 {
		return s.Commit(ctx, before, restored, subject)
	}
	return s.CommitProjection(ctx, before, restored, subject, clients...)
}

// validateTransition refuses a configuration that cannot be saved before the
// guards that assume a valid one, and before anything is written.
func validateTransition(after domain.Config) error {
	candidate := after.Clone()
	candidate.Normalize()
	if err := candidate.Validate(); err != nil {
		return fmt.Errorf("refuse invalid config: %w", err)
	}
	return nil
}

// PreflightError reports a transition that was refused before anything was
// written, by a command or by its preview: the configuration and every
// client file are as they were. It reads as the refusal behind it, which
// names the step that resolves it; a presentation adds that nothing changed.
type PreflightError struct {
	Err error
}

// Error returns the refusal's own text.
func (e *PreflightError) Error() string { return e.Err.Error() }

// Unwrap returns the refusal.
func (e *PreflightError) Unwrap() error { return e.Err }

// Unchanged reports that the configuration and the client files are as they
// were before the command.
func (*PreflightError) Unchanged() bool { return true }

// committed is what a transaction changed: a client file or AIGW's record of
// one, and the stored configuration.
type committed struct {
	projection, configuration bool
}

func (s Synchronizer) commit(ctx context.Context, before, after domain.Config, subject string, reconcileProjection bool, clientIDs ...string) (committed, error) {
	if err := ctx.Err(); err != nil {
		return committed{}, err
	}
	if err := validateTransition(after); err != nil {
		return committed{}, err
	}
	configBefore, err := s.Config.CaptureSnapshot()
	if err != nil {
		return committed{}, err
	}
	// before is the working configuration that the command read: the stored
	// one with the clients' own choices followed.
	settled, err := s.settle(configBefore, before, after, reconcileProjection, clientIDs)
	if !settled.matched {
		return committed{}, errors.Join(errors.New("configuration preimage changed; refusing to overwrite newer state"), err)
	}
	if err != nil {
		return committed{}, err
	}
	after, followed := settled.after, settled.followed
	projection, err := planned(s, before, after, settled.reconcile, settled.clients, settled.left)
	if err != nil {
		return committed{}, err
	}
	// The credential reader is prepared for the clients whose files are
	// written; a client of which only AIGW's record is written reads none.
	projectable := projection.Clients
	if err := ctx.Err(); err != nil {
		return committed{}, err
	}
	var undoEntrypoint func() error
	var entrypoint *secret.Entrypoint
	if len(projectable) > 0 {
		entrypoint, undoEntrypoint, err = s.prepareCredentialEntrypoint(after, projectable...)
		if err != nil {
			return committed{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return committed{}, errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
	}
	configAfter, err := s.Config.Commit(configBefore, after)
	if err != nil {
		return committed{}, errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
	}
	result := committed{configuration: !configAfter.Config.Equal(configBefore.Config)}
	if len(s.reconciled(projection)) == 0 {
		s.Follows.settle(s.keeps(followed, before, after))
		return result, nil
	}
	receipt, projectionChanged, err := s.applyProjection(ctx, before, after, configBefore, configAfter, undoEntrypoint, projection)
	if err != nil {
		return committed{}, fmt.Errorf("%s %w", subject, err)
	}
	if err := s.finalizeCredentialEntrypoint(entrypoint); err != nil {
		projectionErr := receipt.Rollback()
		configErr := s.Config.RestoreSnapshot(configBefore, configAfter)
		if projectionErr != nil {
			projectionErr = platform.FailedUndo("restoring the client files", fmt.Errorf("%w: %w", adapter.ErrProjectionRollbackFailed, projectionErr))
		}
		var entrypointErr error
		if projectionErr == nil && configErr == nil {
			entrypointErr = undoCreatedEntrypoint(undoEntrypoint)
		}
		if rollbackErr := errors.Join(projectionErr, configErr, entrypointErr); rollbackErr != nil {
			return committed{}, projectionError{
				cause:    fmt.Errorf("%s credential entrypoint finalization failed: %w; compensation incomplete: %w", subject, err, rollbackErr),
				restored: configErr == nil,
			}
		}
		return committed{}, projectionError{
			cause:    fmt.Errorf("%s credential entrypoint finalization failed; configuration and client files were rolled back: %w", subject, err),
			restored: true,
		}
	}
	result.projection = projectionChanged
	s.Follows.settle(s.keeps(followed, before, after))
	return result, nil
}

// finalizeCredentialEntrypoint confirms, after projection, the reader that this
// transaction prepared; without a prepared reader there is nothing to confirm.
func (s Synchronizer) finalizeCredentialEntrypoint(entrypoint *secret.Entrypoint) error {
	if entrypoint == nil {
		return nil
	}
	return entrypoint.Confirm()
}

func (s Synchronizer) applyProjection(
	ctx context.Context,
	before, after domain.Config,
	configBefore, configAfter store.Snapshot,
	undoEntrypoint func() error,
	projection Projection,
) (adapter.ProjectionReceipt, bool, error) {
	dependencies, err := s.projectionDependencies(projection, before, after)
	var receipt adapter.ProjectionReceipt
	var changed bool
	if err == nil {
		receipt, changed, err = s.registry().Apply(ctx, dependencies, before, after, s.reconciled(projection)...)
	}
	if err != nil {
		if rollbackErr := s.Config.RestoreSnapshot(configBefore, configAfter); rollbackErr != nil {
			return nil, false, projectionError{cause: fmt.Errorf("synchronization failed: %w; rollback also failed: %w", err, rollbackErr)}
		}
		if !errors.Is(err, adapter.ErrProjectionRollbackFailed) {
			err = errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
		}
		return nil, false, projectionError{cause: fmt.Errorf("synchronization failed; configuration was rolled back: %w", err), restored: true}
	}
	return receipt, changed, nil
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

// undoCreatedEntrypoint removes the credential reader that the transaction
// installed, and reports a removal that failed as a failed undo, which a
// failure that joins it keeps recognizable.
func undoCreatedEntrypoint(undo func() error) error {
	if undo == nil {
		return nil
	}
	return platform.FailedUndo("removing the credential reader that this command installed", undo())
}
