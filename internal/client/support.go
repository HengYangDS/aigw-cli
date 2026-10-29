package client

import (
	"context"
	"errors"
	"fmt"

	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secret"
)

// ResolveExecutable keeps a configured executable that is still available and
// otherwise adopts the discovered one.
func ResolveExecutable(clientID, configured, discovered string) (string, error) {
	available, err := discovery.ExecutableAvailable(configured)
	if err != nil {
		return "", fmt.Errorf("inspect configured %s executable: %w", clientID, err)
	}
	if available || discovered == "" {
		return configured, nil
	}
	return discovered, nil
}

// SecretAvailable reports whether the credential store holds an Account's token.
func SecretAvailable(store secret.Store, accountID string) (bool, error) {
	if store == nil {
		return false, nil
	}
	return store.Exists(accountID)
}

// RetainedDefaultReader returns the credential reader that a client's native
// projection still names when it is a valid retained AIGW reader, and the
// current one otherwise. An external reader is kept as configured.
func RetainedDefaultReader(current string, external bool, observe func() (string, error)) (string, error) {
	if current == "" || external || !secret.IsVersionedEntrypointPath(current) {
		return current, nil
	}
	projected, err := observe()
	if err != nil || projected == current {
		return current, err
	}
	if secret.IsVersionedEntrypointPath(projected) {
		if err := secret.ValidateRetainedEntrypoint(current, projected); err != nil {
			return current, err
		}
	} else {
		available, err := discovery.ExecutableAvailable(projected)
		if err != nil || !available {
			return current, errors.New("retained public credential executable is unavailable")
		}
	}
	return projected, nil
}

// SuppressExternalCredentialDiagnostics wraps a verification runner for a
// client that reads an external credential helper: failures suppress the
// helper's diagnostics, and the verifier's response-marker semantics stay.
func SuppressExternalCredentialDiagnostics(runner process.VerificationRunner) process.VerificationRunner {
	return externalCredentialRunner{runner: runner}
}

type externalCredentialRunner struct{ runner process.VerificationRunner }

func (runner externalCredentialRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	output, diagnostic, err := runner.RunCaptureStreams(ctx, plan)
	if err != nil {
		return diagnostic, err
	}
	return output, nil
}

func (runner externalCredentialRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, diagnostic, err := runner.runner.RunCaptureStreams(ctx, plan)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("external credential client failed; diagnostics suppressed")
	}
	if process.DiagnosticFailure(diagnostic) {
		return output, []byte("warning or error: external credential client diagnostics suppressed"), nil
	}
	return output, nil, nil
}
