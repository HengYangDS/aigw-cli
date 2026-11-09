package invocation

import (
	"errors"
	"net/http"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// sameCommand is a retry whose command line would need quoting.
const sameCommand = "Run the same command again."

// ArgumentError is an argument that a command refuses while it runs, such as
// an omitted one that the configuration cannot supply. The CLI root makes it a
// usage error of the command that ran.
type ArgumentError struct{ Err error }

// Error is the refusal's message.
func (e *ArgumentError) Error() string { return e.Err.Error() }

// Unwrap returns the refusal.
func (e *ArgumentError) Unwrap() error { return e.Err }

// MarkUsageErrors makes the argument refusal of every command in the tree
// under root a usage error of that command, whose help is its fix.
func MarkUsageErrors(root *cobra.Command) {
	if validate := root.Args; validate != nil {
		root.Args = func(cmd *cobra.Command, args []string) error {
			if err := validate(cmd, args); err != nil {
				return terminal.Usage(cmd.CommandPath(), err)
			}
			return nil
		}
	}
	for _, child := range root.Commands() {
		MarkUsageErrors(child)
	}
}

// Retry is the fix that runs cmd again as it was given, every flag included:
// its command line, or prose when a word would need quoting.
func Retry(cmd *cobra.Command, args []string) string {
	if line, isPlain := commandLine(cmd, args); isPlain {
		return line
	}
	return sameCommand
}

// promptError is the failure of one prompt, with the prompt's name.
type promptError struct {
	prompt string
	err    error
}

func (e promptError) Error() string { return e.err.Error() }

func (e promptError) Unwrap() error { return e.err }

// Prompted names the prompt whose failure err is, which CancelledPrompt
// names when the prompt ended without an answer. A nil err stays nil.
func Prompted(prompt string, err error) error {
	if err == nil {
		return nil
	}
	return promptError{prompt: prompt, err: err}
}

// CancelledPrompt explains a prompt, named by Prompted, that ended without an
// answer before the command changed anything: cancelled, at the end of its
// input, or with answers that named no choice. It names the prompt, says
// that nothing changed, and names cmd again as it was given, which asks
// again, as its fix. Every other error, a prompt that could not be shown or
// read included, returns unchanged.
func CancelledPrompt(cmd *cobra.Command, args []string, err error) error {
	failed, prompted := errors.AsType[promptError](err)
	if !prompted || !errors.Is(err, terminal.ErrInputCancelled) {
		return err
	}
	title := failed.prompt + " was cancelled"
	if errors.Is(err, terminal.ErrNoChoice) {
		title = failed.prompt + " ended without a choice"
	}
	return terminal.ProblemError(title, "", "Nothing changed.", Retry(cmd, args), err)
}

// ConfigurationBusy is the problem of a command that another AIGW command kept
// from the configuration for the whole wait: it changed nothing, and it runs
// as given once that command finishes.
func ConfigurationBusy(cmd *cobra.Command, args []string, err error) error {
	fix := "After the other aigw command finishes, run the same command again."
	if line, isPlain := commandLine(cmd, args); isPlain {
		fix = "After the other aigw command finishes, rerun `" + line + "`."
	}
	return terminal.ProblemError("Another AIGW command holds the configuration",
		"The configuration lock stayed held while this command waited.", "This command changed nothing.", fix, err)
}

// TokenRejection gives a Token that the provider rejected, with HTTP 401 or
// 403, the fix that runs the command that sent it again with a Token that
// the provider accepts. Any other error returns unchanged.
func TokenRejection(cmd *cobra.Command, err error) error {
	if _, rejected := errors.AsType[*probe.TokenRejectedError](err); !rejected {
		return err
	}
	return rejectedTokenError{error: err, fix: acceptedTokenFix(cmd, cmd.Flags().Args(), "the provider")}
}

// rejectedTokenError is a failure whose fix is its command with another Token.
type rejectedTokenError struct {
	error
	fix string
}

func (e rejectedTokenError) Unwrap() error { return e.error }

// NextAction is the command again with a Token that the provider accepts.
func (e rejectedTokenError) NextAction() string { return e.fix }

// acceptedTokenFix is the fix of a Token that provider rejected: the command
// again with a Token that provider accepts, piped in when the command reads
// its Token from standard input and entered at its prompt otherwise. Run with
// the same Token, the command fails the same way, so a rejection, unlike an
// endpoint that did not answer, never names the same command alone.
func acceptedTokenFix(cmd *cobra.Command, args []string, provider string) string {
	command := "the same command"
	if line, isPlain := commandLine(cmd, args); isPlain {
		command = "`" + line + "`"
	}
	if flag := cmd.Flags().Lookup("token-stdin"); flag != nil && flag.Changed {
		return "Pipe a Token that " + provider + " accepts into " + command + "."
	}
	return "Run " + command + " again and enter a Token that " + provider + " accepts."
}

// TokenValidationProblem explains a Token that a command did not store,
// because its provider rejected it or the request that validates it did not
// complete. A rejected Token needs the same command again with a Token that
// the provider accepts, or, from the read-only environment backend, a new
// value of its variable first. An endpoint that did not answer and a status
// that a later request may not repeat need the same command again; any other
// status points at the endpoint, which endpointFix corrects for the protocol
// that answered.
// The impact names the provider's previous Token only when secrets holds one.
func TokenValidationProblem(cmd *cobra.Command, args []string, secrets secret.Store, provider string, endpointFix func(domain.EndpointProtocol) string, err error) error {
	readOnly := secret.IsReadOnly(secrets)
	impact := "No Token was stored."
	if previous, existsErr := secrets.Exists(provider); readOnly {
		impact = "Nothing changed."
	} else if existsErr == nil && previous {
		impact = "No Token was stored; the previous Token is unchanged."
	}
	fix := "aigw check"
	status, answered := errors.AsType[*probe.EndpointStatusError](err)
	_, rejected := errors.AsType[*probe.TokenRejectedError](err)
	_, unreachable := errors.AsType[*probe.UnreachableError](err)
	switch {
	case answered && !TransientStatus(status.Status):
		fix = endpointFix(status.Protocol)
	case rejected && readOnly:
		fix = "Set " + secret.EnvironmentKey(provider) + " to a Token that the provider accepts, then run the same command again."
		if line, isPlain := commandLine(cmd, args); isPlain {
			fix = "Set " + secret.EnvironmentKey(provider) + " to a Token that the provider accepts, then run `" + line + "`."
		}
	case rejected:
		fix = acceptedTokenFix(cmd, args, "provider "+provider)
	case answered, unreachable:
		fix = Retry(cmd, args)
	}
	return terminal.ProblemError("Token validation failed: "+err.Error(), "", impact, fix, err)
}

// WithImpact is err's own problem with impact added after the impact that it
// states, if any, such as what became of a Token that the failed command had
// obtained.
func WithImpact(err error, impact string) error {
	problem := terminal.ProblemFor(err)
	return terminal.ProblemError(problem.Title, problem.Evidence, strings.TrimSpace(problem.Impact+" "+impact), problem.Fix, err)
}

// TransientStatus reports a status that a later request may not repeat: a
// timeout, a rate limit or a server error. The same command is its fix.
func TransientStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}
