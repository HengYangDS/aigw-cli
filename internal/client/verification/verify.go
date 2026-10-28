// Package verification owns explicit, quota-consuming live model probes.
package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"
)

// ProtocolTimeout bounds each native turn, including cold client startup.
const ProtocolTimeout = time.Minute

const responseSentinel = "AIGW_OK"

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

// VerifyClaudeRuntime performs one bounded Claude CLI request. explicitRoute is
// the invocation's --route, which its retry advice repeats.
func VerifyClaudeRuntime(ctx context.Context, runner process.VerificationRunner, executable, settingsPath string, clientRuntime configuration.Runtime, token, explicitRoute string) error {
	if clientRuntime.Model == "" {
		return fmt.Errorf("Route %q has no Claude model", clientRuntime.RouteID)
	}
	plan, err := claude.VerificationPlan(executable, settingsPath, "Reply with exactly: AIGW_OK", os.Environ(), clientRuntime)
	if err != nil {
		return err
	}
	if runner == nil {
		return fmt.Errorf("Claude verification runner is unavailable")
	}
	output, diagnostic, err := runner.RunCaptureStreams(ctx, plan)
	result, resultErr := readClaudeResult(output)
	if err != nil {
		if result.status != 0 && errors.Is(err, context.DeadlineExceeded) {
			return requestFailureError{message: fmt.Sprintf("Claude minimal verification request failed: client verification timed out after endpoint HTTP %d; run `%s`", result.status, Command(configuration.ClientClaude, explicitRoute)), cause: err}
		}
		return Failure("Claude", configuration.ClientClaude, explicitRoute, bytes.Join([][]byte{diagnostic, output}, []byte("\n")), err, token)
	}
	if process.DiagnosticFailure(diagnostic) {
		return Diagnostic("Claude", diagnostic)
	}
	if resultErr != nil {
		return resultErr
	}
	if result.failed {
		return Failure("Claude", configuration.ClientClaude, explicitRoute, output, errors.New("native Claude result reports an error"), token)
	}
	if strings.TrimSpace(result.response) != responseSentinel {
		return fmt.Errorf("Claude model response did not return the expected AIGW_OK verification marker")
	}
	return nil
}

type claudeResult struct {
	response string
	failed   bool
	status   int
}

func readClaudeResult(output []byte) (claudeResult, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	result := claudeResult{}
	completed := false
	fallback := false
	for {
		var event struct {
			Type        string `json:"type"`
			Subtype     string `json:"subtype"`
			Result      string `json:"result"`
			IsError     *bool  `json:"is_error"`
			ErrorStatus int    `json:"error_status"`
		}
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return result, errors.New("invalid native Claude result")
		}
		switch {
		case event.Type == "system" && event.Subtype == "model_refusal_fallback":
			fallback = true
		case event.Type == "system" && event.Subtype == "api_retry":
			if event.ErrorStatus >= 400 && event.ErrorStatus <= 599 {
				result.status = event.ErrorStatus
			}
		case event.Type == "result":
			if completed || event.IsError == nil {
				return result, errors.New("invalid native Claude result")
			}
			completed = true
			result.response, result.failed = event.Result, *event.IsError
		}
	}
	if !completed {
		return result, errors.New("Claude model response did not return the expected AIGW_OK verification marker")
	}
	if fallback {
		return result, errors.New("Claude selected model was not qualified: native client reported model refusal fallback")
	}
	return result, nil
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

// Failure is the verification request failure that names its retry command.
func Failure(label, client, explicitRoute string, diagnostic []byte, cause error, secrets ...string) error {
	detail := verificationFailureSummary(client, diagnostic, cause, secrets...)
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

func verificationFailureSummary(client string, diagnostic []byte, cause error, secrets ...string) string {
	text := strings.ToLower(redaction.Text(string(diagnostic), secrets...))
	switch {
	case errors.Is(cause, context.DeadlineExceeded), strings.Contains(text, "deadline exceeded"), strings.Contains(text, "timed out"):
		return "client verification timed out"
	case client == configuration.ClientClaude && strings.Contains(text, "context_management") && strings.Contains(text, "extra inputs are not permitted"):
		return "endpoint rejects pre-release context management; set CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1 in native Claude settings env and verify again"
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
