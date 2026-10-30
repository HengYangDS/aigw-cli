// Package balance implements `aigw balance`, which reports an Account's
// balance and Token quota through its provider's precise diagnostics.
package balance

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/provider"
	"aigw-cli/internal/provider/diagnostic"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// diagnosticCredentialProblem explains why precise balance cannot run and names
// a recovery the selected credential backend can perform. The read-only
// environment backend cannot store diagnostics, so it names their variables.
func diagnosticCredentialProblem(store secret.Store, accountName string, err error) error {
	environment := ""
	if secret.IsReadOnly(store) {
		environment = "Set both `" + secret.DiagnosticSystemTokenEnvironmentKey(accountName) + "` and `" + secret.DiagnosticUserIDEnvironmentKey(accountName) + "` in the parent environment, then retry."
	}
	if !errors.Is(err, secret.ErrNotFound) {
		return terminal.ProblemError(
			"Cannot read provider diagnostic credential",
			"AIGW could not read a usable provider diagnostic credential for "+accountName+"; this does not establish that it is missing.",
			"No balance request was sent; the Account Token and diagnostic credential were not changed.",
			cmp.Or(environment, "Run `aigw doctor` to inspect the selected credential backend; if it is available, explicitly re-enable diagnostics with `aigw account diagnostics enable "+accountName+"`."),
			err,
		)
	}
	return terminal.ProblemError(
		"Precise balance diagnostics are not enabled",
		"Missing "+accountName+" provider platform diagnostic credential; the Account Token is stored separately in the selected credential backend.",
		"Cannot distinguish account balance, remaining token quota, disabled token state, and request limits.",
		cmp.Or(environment, "aigw account diagnostics enable "+accountName),
		err,
	)
}

// NewCommand constructs the explicit provider-account diagnostic command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "balance [account]", Short: "Show account balance and token quota", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		name, err := invocation.AccountReference(cfg, args)
		if err != nil {
			return err
		}
		accountName, providerAccount, err := cfg.ResolveAccount(name)
		if err != nil {
			return err
		}
		if providerAccount.AccountProbe == nil {
			return fmt.Errorf("%s does not support precise balance queries", providerAccount.Label)
		}
		if !provider.Supports(providerAccount.AccountProbe.Kind) {
			return fmt.Errorf("Precise diagnostics provider %q is not included in this AIGW version; continue using `aigw check` for general diagnostics", providerAccount.AccountProbe.Kind)
		}
		credential, err := runtime.Accounts.Get(accountName)
		if err != nil {
			return diagnosticCredentialProblem(runtime.Secrets, accountName, err)
		}
		apiToken, err := runtime.Secrets.Get(accountName)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		providerAccount.ID = accountName
		report, err := provider.Probe(ctx, probe.Sender{Client: runtime.HTTP}, providerAccount, apiToken, diagnostic.Credential(credential))
		if err != nil {
			return err
		}
		status := map[string]string{"enabled": "Enabled", "disabled": "Disabled"}[report.TokenStatus]
		remaining := fmt.Sprintf("$%.4f", report.TokenRemaining)
		if report.TokenUnlimitedQuota {
			remaining = "Unlimited"
		}
		count := fmt.Sprintf("%d requests", report.TokenRemainingCount)
		if report.TokenUnlimitedCount {
			count = "Unlimited requests"
		}
		r := invocation.Renderer(runtime)
		r.ProductTitle("Account and quota")
		r.Section("Account")
		r.Rows(
			terminal.Field{Label: "Account", Value: providerAccount.Label},
			terminal.Field{Label: "Account balance", Value: fmt.Sprintf("$%.4f", report.AccountBalance)},
		)
		r.Section("Current API token")
		state := terminal.OK
		if report.TokenStatus != "enabled" {
			state = terminal.Fail
		}
		r.Rows(
			terminal.Field{Label: "Name", Value: report.TokenName},
			terminal.Field{State: state, Label: "Token status", Value: status},
			terminal.Field{Label: "Quota used", Value: fmt.Sprintf("$%.4f", report.TokenUsed)},
			terminal.Field{Label: "Remaining quota", Value: remaining},
			terminal.Field{Label: "Remaining requests", Value: count},
		)
		r.Next("aigw check")
		return nil
	}}
}
