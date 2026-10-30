package synchronization

import (
	"context"
	"errors"
	"fmt"

	"aigw-cli/internal/client"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// Commit persists one configuration transition and converges affected client
// projections. Failures compensate owned writes without changing native credentials.
func (s Synchronizer) Commit(ctx context.Context, before, after domain.Config, subject string) error {
	clients := s.registry().ChangedClients(before, after)
	_, err := s.commit(ctx, before, after, subject, len(clients) > 0, domain.ConnectionChange{}, clients...)
	return err
}

// CommitProjection persists configuration and reconciles every client projection,
// including missing or out-of-date projections of unchanged configuration.
func (s Synchronizer) CommitProjection(ctx context.Context, before, after domain.Config, subject string, clientIDs ...string) error {
	_, err := s.commit(ctx, before, after, subject, true, domain.ConnectionChange{}, clientIDs...)
	return err
}

func (s Synchronizer) commit(ctx context.Context, before, after domain.Config, subject string, reconcileProjection bool, connection domain.ConnectionChange, clientIDs ...string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// A configuration that cannot be saved is refused before the guards that
	// assume a valid one, and before anything is written.
	candidate := after.Clone()
	candidate.Normalize()
	if err := candidate.Validate(); err != nil {
		return false, fmt.Errorf("refuse invalid config: %w", err)
	}
	if err := s.validateRetryTransition(before, after, connection); err != nil {
		return false, err
	}
	configBefore, err := s.Config.CaptureSnapshot()
	if err != nil {
		return false, err
	}
	matched, err := configBefore.MatchesConfiguration(before)
	if err != nil || !matched {
		return false, errors.Join(errors.New("configuration preimage changed; refusing to overwrite newer state"), err)
	}
	var projectable []string
	if reconcileProjection {
		projectable, err = s.credentialReadyClients(after, clientIDs...)
		if err != nil {
			return false, fmt.Errorf("%s synchronization preflight failed; configuration and client files were unchanged: %w", subject, err)
		}
	}
	if len(projectable) > 0 {
		dependencies, err := s.clientDependencies(projectable, before, after)
		if err != nil {
			return false, err
		}
		if _, err := s.registry().Plan(dependencies, before, after, projectable...); err != nil {
			return false, fmt.Errorf("%s synchronization preflight failed; configuration and client files were unchanged: %w", subject, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var undoEntrypoint func() error
	var entrypoint *secret.Entrypoint
	if len(projectable) > 0 {
		entrypoint, undoEntrypoint, err = s.prepareCredentialEntrypoint(after, projectable...)
		if err != nil {
			return false, err
		}
	}
	if err := ctx.Err(); err != nil {
		return false, errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
	}
	configAfter, err := s.Config.Commit(configBefore, after)
	if err != nil {
		return false, errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
	}
	if len(projectable) == 0 {
		return false, nil
	}
	receipt, projectionChanged, err := s.applyProjection(ctx, before, after, configBefore, configAfter, undoEntrypoint, projectable...)
	if err != nil {
		return false, fmt.Errorf("%s %w", subject, err)
	}
	if err := s.finalizeCredentialEntrypoint(entrypoint); err != nil {
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
			return false, projectionError{
				cause:    fmt.Errorf("%s credential entrypoint finalization failed: %w; compensation incomplete: %w", subject, err, rollbackErr),
				restored: configErr == nil,
			}
		}
		return false, projectionError{
			cause:    fmt.Errorf("%s credential entrypoint finalization failed; configuration and client projections were rolled back: %w", subject, err),
			restored: true,
		}
	}
	return projectionChanged, nil
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
	clientIDs ...string,
) (client.ProjectionReceipt, bool, error) {
	dependencies, err := s.clientDependencies(clientIDs, before, after)
	var receipt client.ProjectionReceipt
	var changed bool
	if err == nil {
		receipt, changed, err = s.registry().Apply(ctx, dependencies, before, after, clientIDs...)
	}
	if err != nil {
		if rollbackErr := s.Config.RestoreSnapshot(configBefore, configAfter); rollbackErr != nil {
			return nil, false, projectionError{cause: fmt.Errorf("synchronization failed: %w; rollback also failed: %w", err, rollbackErr)}
		}
		if !errors.Is(err, client.ErrProjectionRollbackFailed) {
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

func undoCreatedEntrypoint(undo func() error) error {
	if undo == nil {
		return nil
	}
	return undo()
}
