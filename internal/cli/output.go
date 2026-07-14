package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

type userError struct {
	problem presentation.Problem
	cause   error
}

func (e *userError) Error() string { return e.problem.Title }
func (e *userError) Unwrap() error { return e.cause }

type presentedError struct{ cause error }

func (e *presentedError) Error() string { return e.cause.Error() }
func (e *presentedError) Unwrap() error { return e.cause }

func renderer(app *App) *presentation.Renderer {
	return presentation.New(app.Out, app.Color)
}

func problem(title, evidence, impact, fix string, cause error) error {
	return &userError{problem: presentation.Problem{Title: title, Evidence: evidence, Impact: impact, Fix: fix}, cause: cause}
}

func presented(err error) error { return &presentedError{cause: err} }

func RenderError(app *App, err error) {
	var already *presentedError
	if errors.As(err, &already) {
		return
	}
	var user *userError
	if errors.As(err, &user) {
		renderer(app).Problem(user.problem)
		return
	}
	message := localizeCobraError(err.Error())
	renderer(app).Problem(presentation.Problem{
		Title:  message,
		Impact: "The operation did not complete; any committed transaction will be rolled back where possible.",
		Fix:    suggestedFix(message),
	})
}

func localizeCobraError(message string) string {
	if command, ok := cobraUnknownCommand(message); ok {
		return fmt.Sprintf("unknown command %q", command)
	}
	if flag, ok := cobraUnknownFlag(message); ok {
		return fmt.Sprintf("unknown option --%s", flag)
	}
	if profile, expected, actual, ok := runtimeProfileClientMismatch(message); ok {
		return fmt.Sprintf("profile %q is for %s, not %s", profile, expected, actual)
	}
	if profile, account, ok := runtimeProfileUnknownAccount(message); ok {
		return fmt.Sprintf("profile %q references unknown account %q", profile, account)
	}
	if account, ok := runtimeMissingEndpoint(message, "Anthropic"); ok {
		return fmt.Sprintf("account %q has no Anthropic endpoint", account)
	}
	if account, ok := runtimeMissingEndpoint(message, "OpenAI Responses"); ok {
		return fmt.Sprintf("account %q has no OpenAI Responses endpoint", account)
	}
	if version, expected, ok := unsupportedConfigVersion(message); ok {
		return fmt.Sprintf("unsupported configuration version: found %s, expected %s", version, expected)
	}
	if configLoadFailure(message) {
		return "Cannot read or validate local configuration; run `aigw doctor` to inspect or restore it"
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

func runtimeProfileClientMismatch(message string) (profile, expected, actual string, ok bool) {
	const prefix = `profile "`
	if !strings.HasPrefix(message, prefix) {
		return "", "", "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	profile, rest, found := strings.Cut(rest, `" is for `)
	if !found || profile == "" {
		return "", "", "", false
	}
	expected, actual, found = strings.Cut(rest, ", not ")
	if !found || expected == "" || actual == "" {
		return "", "", "", false
	}
	return profile, expected, actual, true
}

func runtimeProfileUnknownAccount(message string) (profile, account string, ok bool) {
	const prefix = `profile "`
	if !strings.HasPrefix(message, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	profile, rest, found := strings.Cut(rest, `" references unknown account "`)
	if !found || profile == "" || !strings.HasSuffix(rest, `"`) {
		return "", "", false
	}
	account = strings.TrimSuffix(rest, `"`)
	if account == "" {
		return "", "", false
	}
	return profile, account, true
}

func runtimeMissingEndpoint(message, protocol string) (account string, ok bool) {
	const prefix = `account "`
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	account, rest, found := strings.Cut(rest, `" has no `+protocol+` endpoint`)
	if !found || account == "" || rest != "" {
		return "", false
	}
	return account, true
}

func unsupportedConfigVersion(message string) (version, expected string, ok bool) {
	const marker = "validate config: unsupported config version "
	start := strings.Index(message, marker)
	if start < 0 {
		return "", "", false
	}
	rest := message[start+len(marker):]
	version, rest, found := strings.Cut(rest, "; expected ")
	if !found || version == "" || rest == "" {
		return "", "", false
	}
	if _, err := strconv.Atoi(version); err != nil {
		return "", "", false
	}
	if _, err := strconv.Atoi(rest); err != nil {
		return "", "", false
	}
	return version, rest, true
}

// configLoadFailure recognizes errors from AIGW's local configuration boundary.
// Human output intentionally omits parser diagnostics and filesystem paths;
// `aigw doctor --json` remains the detailed diagnostic surface.
func configLoadFailure(message string) bool {
	for _, marker := range []string{"read config:", "parse config:", "validate config:"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
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

func healthImpact(cfgClients int) string {
	switch cfgClients {
	case 0:
		return "The current API route is unavailable."
	case 1:
		return "The configured AI client is unavailable."
	default:
		return fmt.Sprintf("%d configured AI clients are unavailable.", cfgClients)
	}
}
