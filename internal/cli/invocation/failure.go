package invocation

import (
	"errors"
	"net/http"

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
// 403, the exact retry of the command that sent it: the same command line,
// which reads a Token again as it did. Any other error returns unchanged.
func TokenRejection(cmd *cobra.Command, err error) error {
	if _, rejected := errors.AsType[*probe.TokenRejectedError](err); !rejected {
		return err
	}
	return retryable{error: err, retry: Retry(cmd, cmd.Flags().Args())}
}

// retryable is a failure whose fix is to run its command again.
type retryable struct {
	error
	retry string
}

func (e retryable) Unwrap() error { return e.error }

// NextAction is the retry of the failed command.
func (e retryable) NextAction() string { return e.retry }

// TokenValidationProblem explains a Token that a command did not store,
// because its provider rejected it or the request that validates it did not
// complete. A rejected Token, an endpoint that did not answer and a status that
// a later request may not repeat need the same command again; any other status
// points at the endpoint, which endpointFix corrects for the protocol that
// answered. The impact names the provider's previous Token only when secrets
// holds one.
func TokenValidationProblem(cmd *cobra.Command, args []string, secrets secret.Store, provider string, endpointFix func(domain.EndpointProtocol) string, err error) error {
	impact := "No Token was stored."
	if previous, existsErr := secrets.Exists(provider); existsErr == nil && previous {
		impact = "No Token was stored; the previous Token is unchanged."
	}
	fix := "aigw check"
	status, answered := errors.AsType[*probe.EndpointStatusError](err)
	_, rejected := errors.AsType[*probe.TokenRejectedError](err)
	_, unreachable := errors.AsType[*probe.UnreachableError](err)
	switch {
	case answered && !TransientStatus(status.Status):
		fix = endpointFix(status.Protocol)
	case answered, rejected, unreachable:
		fix = Retry(cmd, args)
	}
	return terminal.ProblemError("Token validation failed: "+err.Error(), "", impact, fix, err)
}

// TransientStatus reports a status that a later request may not repeat: a
// timeout, a rate limit or a server error. The same command is its fix.
func TransientStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}
