package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
)

const responseSentinel = "AIGW_OK"

// failureAdvice explains an endpoint's refusal of Claude Code's pre-release
// context management.
var failureAdvice = []clientverification.Advice{{
	Markers: []string{"context_management", "extra inputs are not permitted"},
	Summary: "endpoint rejects pre-release context management; set CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1 in native Claude settings env and verify again",
}}

// verifyRuntime performs one bounded Claude CLI request. explicitRoute is
// the invocation's --route, which its retry advice repeats.
func verifyRuntime(ctx context.Context, runner process.VerificationRunner, executable, settingsPath string, clientRuntime domain.Runtime, token, explicitRoute string) error {
	if clientRuntime.Model == "" {
		return fmt.Errorf("Route %q has no Claude model", clientRuntime.RouteID)
	}
	plan, err := VerificationPlan(executable, settingsPath, "Reply with exactly: AIGW_OK", os.Environ(), clientRuntime)
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
			return clientverification.RequestFailure(fmt.Sprintf("Claude minimal verification request failed: client verification timed out after endpoint HTTP %d; run `%s`", result.status, clientverification.Command(ID, explicitRoute)), err)
		}
		return clientverification.Failure("Claude", ID, explicitRoute, bytes.Join([][]byte{diagnostic, output}, []byte("\n")), err, failureAdvice, token)
	}
	if process.DiagnosticFailure(diagnostic) {
		return clientverification.Diagnostic("Claude", diagnostic)
	}
	if resultErr != nil {
		return resultErr
	}
	if result.failed {
		return clientverification.Failure("Claude", ID, explicitRoute, output, errors.New("native Claude result reports an error"), failureAdvice, token)
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
