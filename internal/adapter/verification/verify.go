// Package verification holds what every client's live model probe shares: the
// turn deadline, bounded response reads, failure messages and the retry command.
package verification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"aigw-cli/internal/domain"
)

// ProtocolTimeout bounds each native turn, including cold client startup.
const ProtocolTimeout = time.Minute

// ReadBoundedFile reads a client's response file and refuses one larger than limit.
func ReadBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

// Diagnostic explains a native client's warning or error output, which leaves
// verification unqualified.
func Diagnostic(label string, diagnostic []byte) error {
	text := strings.ToLower(string(diagnostic))
	if strings.Contains(text, "model metadata") && strings.Contains(text, "fallback metadata") {
		return fmt.Errorf("%s emitted a model-metadata warning; native metadata is incomplete and verification is unqualified; select a model supported by the client or update the client", label)
	}
	return fmt.Errorf("%s emitted a native-client warning or error; verification is incomplete; inspect the client with the selected Route", label)
}

type requestFailureError struct {
	message string
	cause   error
}

func (failure requestFailureError) Error() string { return failure.message }

func (failure requestFailureError) Unwrap() error { return failure.cause }

// RequestFailure is a verification request failure with a complete message;
// [errors.Is] still finds its cause.
func RequestFailure(message string, cause error) error {
	return requestFailureError{message: message, cause: cause}
}

// Advice is a client's own explanation of one diagnostic: a failed request
// whose lowercased, redacted diagnostic contains every marker is summarized as
// Summary.
type Advice struct {
	Markers []string
	Summary string
}

// Failure is the verification request failure that names its retry command.
// The client's advice explains its known diagnostics after a timeout and
// before the shared model and authentication summaries.
func Failure(label, client, explicitRoute string, diagnostic []byte, cause error, advice []Advice, secrets ...string) error {
	detail := verificationFailureSummary(diagnostic, cause, advice, secrets...)
	return requestFailureError{
		message: fmt.Sprintf("%s minimal verification request failed: %s; run `%s`", label, detail, Command(client, explicitRoute)),
		cause:   cause,
	}
}

// Command returns the verify invocation for one client. An explicit Route stays
// in it: verification of an explicit Route takes its own isolated path, so a
// retry without --route would verify another target.
func Command(client, explicitRoute string) string {
	command := "aigw verify --for " + client
	if explicitRoute != "" {
		command += " --route " + explicitRoute
	}
	return command
}

func verificationFailureSummary(diagnostic []byte, cause error, advice []Advice, secrets ...string) string {
	text := strings.ToLower(domain.Redact(string(diagnostic), secrets...))
	if errors.Is(cause, context.DeadlineExceeded) || strings.Contains(text, "deadline exceeded") || strings.Contains(text, "timed out") {
		return "client verification timed out"
	}
	for _, item := range advice {
		if !slices.ContainsFunc(item.Markers, func(marker string) bool { return !strings.Contains(text, marker) }) {
			return item.Summary
		}
	}
	switch {
	case strings.Contains(text, "unrecognized_model"),
		strings.Contains(text, "not support for model"),
		strings.Contains(text, "model id") && strings.Contains(text, "incorrect"),
		strings.Contains(text, "model") && strings.Contains(text, "unavailable"),
		strings.Contains(text, "model") && strings.Contains(text, "status 503"):
		return "selected model is unavailable through the client or endpoint"
	case strings.Contains(text, "provider auth command"),
		strings.Contains(text, "unauthorized"),
		strings.Contains(text, "authentication"),
		strings.Contains(text, "status 401"):
		return "client or endpoint rejected authentication"
	default:
		return "client process rejected the request; diagnostics suppressed"
	}
}
