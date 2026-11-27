package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
)

// StateSuffix names the ownership state file that an adapter keeps beside a
// native file it projects.
const StateSuffix = ".aigw-state.json"

// EncodeJSON returns a document that an adapter writes: value as JSON
// indented by two spaces, with a final newline.
func EncodeJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// RawJSON encodes one value for a JSON document that an adapter edits. A
// value that JSON cannot encode yields nil.
func RawJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

// ConvergeBinding is the start that every client adapter's Converge shares.
// It resolves the client's selected Route, its enabled Client Binding and the
// program that the client keeps as its declaration under the one rule of
// ClientProgram: the program that it declares, none for a client that
// declares an extensions directory, which keeps that, or, for a client that
// declares neither, the program that discovery found, which the adapter's
// Converge adopts by writing it as the client's program. It checks that
// AIGW's credential store holds a Token of the selected Account that AIGW can
// use when the Route reads it from there. ready reports whether all of them
// are present; it is false for a client that declares neither and of which
// discovery found none, and the adapter then leaves the client's activation
// unchanged. A declared program that is missing is kept, and the plan stops
// the client (see Registry.PlanStopping). A Token that the store cannot
// observe is no Token that AIGW can use: the client is not ready, and the
// store's failure is no error of the command.
func ConvergeBinding(deps Dependencies, cfg *domain.Config, discovered discovery.Result, clientID string) (binding domain.ClientBinding, program string, ready bool, err error) {
	runtime, err := cfg.ResolveRuntime(clientID, domain.ModelRef{})
	if _, unselected := errors.AsType[*domain.RuntimeBindingUnselectedError](err); unselected {
		return domain.ClientBinding{}, "", false, nil
	}
	if err != nil {
		return domain.ClientBinding{}, "", false, err
	}
	binding, configured := cfg.Clients[clientID]
	if !configured || !binding.Enabled {
		return domain.ClientBinding{}, "", false, nil
	}
	program = binding.Program
	if resolved, _ := ClientProgram(*cfg, clientID, discovered, deps.Host); resolved.Discovered {
		if resolved.Path == "" {
			return domain.ClientBinding{}, "", false, nil
		}
		program = resolved.Path
	}
	available := !runtime.UsesAIGWCredentialStore() || secret.ObserveToken(deps.Secrets, runtime.ProviderID).Usable()
	return binding, program, available, nil
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
