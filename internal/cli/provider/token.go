package provider

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// requireTokenInput refuses, before any configuration access, a command that
// would have to prompt for a Token where no command may: without a terminal,
// or with --json. Its fix pipes the Token to the same command with
// --token-stdin. A preview reads no Token, and the read-only environment
// backend supplies its own or refuses later, so neither needs input.
func requireTokenInput(cmd *cobra.Command, runtime invocation.Context, stdin, jsonMode, dryRun bool) error {
	if stdin || dryRun || secret.IsReadOnly(runtime.Secrets) || runtime.Interactive && !jsonMode {
		return nil
	}
	reason := "a Token is read from --token-stdin without a terminal or with --json"
	if line := invocation.Rerun(cmd, cmd.Flags().Args(), nil, "--token-stdin"); line != "" {
		return fmt.Errorf("%s; pipe the Token to `%s`", reason, line)
	}
	return fmt.Errorf("%s; run `%s --help`", reason, cmd.CommandPath())
}

// readToken reads a secret that a command stores, such as a Token: one value
// from standard input with stdin, and otherwise from prompt at the terminal,
// which never echoes it. Each command refuses a prompt where it may not ask.
func readToken(runtime invocation.Context, prompt string, stdin bool) (string, error) {
	if stdin {
		return invocation.ReadToken(runtime, true, false)
	}
	token, err := runtime.Prompt.Secret(prompt)
	if err != nil {
		return "", err
	}
	return token, invocation.ValidateToken(token)
}

// readOnlyToken refuses a Token that the read-only environment backend would
// have to store, for action: the provider's variable supplies the Token
// instead, so the fix sets it and, when then names one, runs that command.
func readOnlyToken(id, action, then string) error {
	key := secret.EnvironmentKey(id)
	fix := "Set " + key + " in aigw's environment"
	if then != "" {
		fix += ", then run `" + then + "`"
	}
	return terminal.ProblemError(fmt.Sprintf("the environment backend reads the %s Token from %s and cannot %s it", id, key, action), "", "", fix+".", nil)
}
