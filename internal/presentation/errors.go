package presentation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	configuration "aigw-cli/internal/configuration"
)

type userError struct {
	problem Problem
	cause   error
}

func (e *userError) Error() string { return e.problem.Title }
func (e *userError) Unwrap() error { return e.cause }

type presentedError struct{ cause error }

func (e *presentedError) Error() string { return e.cause.Error() }
func (e *presentedError) Unwrap() error { return e.cause }

// ProblemError creates a structured user-facing problem while preserving its underlying cause.
func ProblemError(title, evidence, impact, fix string, cause error) error {
	return &userError{problem: Problem{Title: title, Evidence: evidence, Impact: impact, Fix: fix}, cause: cause}
}

// Presented marks an error whose command result has already been rendered.
func Presented(err error) error { return &presentedError{cause: err} }

// RenderError emits one structured actionable error and records any output failure on the renderer.
func RenderError(renderer *Renderer, err error, jsonMode bool) {
	if _, ok := errors.AsType[*presentedError](err); ok {
		return
	}
	var problem Problem
	if user, ok := errors.AsType[*userError](err); ok {
		problem = user.problem
	} else {
		message := localizedErrorMessage(err)
		problem = Problem{
			Title:  message,
			Impact: "The command could not finish; inspect current state before retrying.",
			Fix:    suggestedFix(message),
		}
	}
	if jsonMode {
		encoder := json.NewEncoder(renderer.out)
		encoder.SetIndent("", "  ")
		renderer.err = encoder.Encode(struct {
			OK bool `json:"ok"`
			Problem
		}{Problem: problem})
		return
	}
	renderer.Problem(problem)
}

func localizedErrorMessage(err error) string {
	if message, ok := typedErrorMessage(err); ok {
		return message
	}
	return localizeCobraError(err.Error())
}

func typedErrorMessage(err error) (string, bool) {
	if mismatch, ok := errors.AsType[*configuration.RuntimeProfileClientMismatchError](err); ok {
		return fmt.Sprintf("profile %q is for %s, not %s", mismatch.ProfileID, mismatch.ExpectedClient, mismatch.ActualClient), true
	}
	if unknownAccount, ok := errors.AsType[*configuration.RuntimeProfileUnknownAccountError](err); ok {
		return fmt.Sprintf("profile %q references unknown account %q", unknownAccount.ProfileID, unknownAccount.AccountID), true
	}
	if missingEndpoint, ok := errors.AsType[*configuration.RuntimeMissingEndpointError](err); ok {
		switch missingEndpoint.Protocol {
		case configuration.ProtocolAnthropic:
			return fmt.Sprintf("account %q has no Anthropic endpoint", missingEndpoint.AccountID), true
		case configuration.ProtocolOpenAIResponses:
			return fmt.Sprintf("account %q has no OpenAI Responses endpoint", missingEndpoint.AccountID), true
		}
	}
	if version, ok := errors.AsType[*configuration.UnsupportedConfigVersionError](err); ok {
		return fmt.Sprintf("unsupported configuration version: found %d, expected %d", version.Version, version.ExpectedVersion), true
	}
	if _, ok := errors.AsType[*configuration.LoadError](err); ok {
		return "Cannot read or validate local configuration; run `aigw doctor` to inspect or restore it", true
	}
	return "", false
}

func localizeCobraError(message string) string {
	if command, ok := cobraUnknownCommand(message); ok {
		return fmt.Sprintf("unknown command %q", command)
	}
	if flag, ok := cobraUnknownFlag(message); ok {
		return fmt.Sprintf("unknown option --%s", flag)
	}
	return message
}

func cobraUnknownCommand(message string) (string, bool) {
	const prefix = `unknown command "`
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	command, suffix, ok := strings.Cut(rest, `" for "`)
	if !ok || !strings.HasSuffix(suffix, `"`) || command == "" {
		return "", false
	}
	return command, true
}

func cobraUnknownFlag(message string) (string, bool) {
	const prefix = "unknown flag: --"
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	flag := strings.TrimSpace(strings.TrimPrefix(message, prefix))
	if flag == "" || strings.ContainsAny(flag, " \t\r\n") {
		return "", false
	}
	return flag, true
}

func suggestedFix(message string) string {
	if command := mentionedAIGWCommand(message); command != "" {
		return command
	}
	switch {
	case strings.Contains(message, "unknown command"), strings.Contains(message, "unknown option"), strings.Contains(message, "unknown flag"):
		return "aigw --help"
	default:
		return "aigw check"
	}
}

func mentionedAIGWCommand(message string) string {
	start := strings.Index(message, "`aigw ")
	if start < 0 {
		return ""
	}
	value := message[start+1:]
	before, _, ok := strings.Cut(value, "`")
	if !ok {
		return ""
	}
	return before
}
