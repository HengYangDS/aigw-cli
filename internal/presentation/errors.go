package presentation

import (
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

func ProblemError(title, evidence, impact, fix string, cause error) error {
	return &userError{problem: Problem{Title: title, Evidence: evidence, Impact: impact, Fix: fix}, cause: cause}
}

func Presented(err error) error { return &presentedError{cause: err} }

func RenderError(renderer *Renderer, err error) {
	var already *presentedError
	if errors.As(err, &already) {
		return
	}
	var user *userError
	if errors.As(err, &user) {
		renderer.Problem(user.problem)
		return
	}
	message := localizedErrorMessage(err)
	renderer.Problem(Problem{
		Title:  message,
		Impact: "The operation did not complete; any committed transaction will be rolled back where possible.",
		Fix:    suggestedFix(message),
	})
}

func localizedErrorMessage(err error) string {
	if message, ok := typedErrorMessage(err); ok {
		return message
	}
	return localizeCobraError(err.Error())
}

func typedErrorMessage(err error) (string, bool) {
	var mismatch *configuration.RuntimeProfileClientMismatchError
	if errors.As(err, &mismatch) {
		return fmt.Sprintf("profile %q is for %s, not %s", mismatch.ProfileID, mismatch.ExpectedClient, mismatch.ActualClient), true
	}
	var unknownAccount *configuration.RuntimeProfileUnknownAccountError
	if errors.As(err, &unknownAccount) {
		return fmt.Sprintf("profile %q references unknown account %q", unknownAccount.ProfileID, unknownAccount.AccountID), true
	}
	var missingEndpoint *configuration.RuntimeMissingEndpointError
	if errors.As(err, &missingEndpoint) {
		switch missingEndpoint.Protocol {
		case configuration.ProtocolAnthropic:
			return fmt.Sprintf("account %q has no Anthropic endpoint", missingEndpoint.AccountID), true
		case configuration.ProtocolOpenAIResponses:
			return fmt.Sprintf("account %q has no OpenAI Responses endpoint", missingEndpoint.AccountID), true
		}
	}
	var version *configuration.UnsupportedConfigVersionError
	if errors.As(err, &version) {
		return fmt.Sprintf("unsupported configuration version: found %d, expected %d", version.Version, version.ExpectedVersion), true
	}
	var load *configuration.LoadError
	if errors.As(err, &load) {
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
	end := strings.Index(value, "`")
	if end < 0 {
		return ""
	}
	return value[:end]
}
