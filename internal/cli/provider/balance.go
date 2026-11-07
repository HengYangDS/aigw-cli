package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"aigw-cli/internal/balanceprobe"
	"aigw-cli/internal/balanceprobe/diagnostic"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// The states of one provider's balance report.
const (
	balanceReported = "reported"
	balanceNoProbe  = "no_probe"
	balanceFailed   = "failed"
)

// balanceTimeout bounds one provider's balance probe.
const balanceTimeout = 15 * time.Second

func newBalanceCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "balance [provider...]",
		Short: "Report the balance and Token quota of every provider, or of those named",
		Long: "Report the balance and Token quota of every provider, or of those named, through each provider's balance probe.\n" +
			"A provider without a balance probe says so and does not fail; a probe that fails hides no other provider.",
		Example:           "aigw provider balance\naigw provider balance dmxapi\naigw provider balance dmxapi ucloud --json",
		ValidArgsFunction: invocation.CompleteProviders(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBalance(cmd, runtime, args, jsonMode)
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the balances as JSON")
	return cmd
}

func runBalance(cmd *cobra.Command, runtime invocation.Context, args []string, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	ids, err := balanceProviders(cfg, args)
	if err != nil {
		return err
	}
	document := balanceDocument{Providers: make([]balanceItem, 0, len(ids))}
	next, failures := "aigw check", 0
	for _, id := range ids {
		item, fix := probeBalance(cmd.Context(), runtime, cfg, id)
		item.NextAction, item.NextDetail = terminal.NextSteps(fix)
		if item.State == balanceFailed {
			if failures == 0 {
				next = fix
			}
			failures++
		}
		document.Providers = append(document.Providers, item)
	}
	var failure error
	if failures > 0 {
		failure = fmt.Errorf("the balance probe of %s failed", count(failures, "provider"))
	}
	if jsonMode {
		document.NextAction, document.NextDetail = terminal.NextSteps(next)
		if err := terminal.WriteDocument(runtime.Out, balanceSchemaVersion, failure == nil, document); err != nil {
			return err
		}
	} else if err := renderBalance(runtime, document, next); err != nil {
		return err
	}
	if failure != nil {
		return terminal.Presented(failure)
	}
	return nil
}

// balanceProviders are the providers that balance reports: those named, each
// once in the order given, or else every provider in configuration order.
func balanceProviders(cfg domain.Config, args []string) ([]string, error) {
	if len(args) == 0 {
		if len(cfg.Providers) == 0 {
			return nil, terminal.ProblemError("no provider is configured", "", "", "aigw setup", nil)
		}
		return cfg.CatalogProviderIDs(), nil
	}
	ids := make([]string, 0, len(args))
	for _, arg := range args {
		provider, err := invocation.ResolveProvider(cfg, arg)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(ids, provider.ID) {
			ids = append(ids, provider.ID)
		}
	}
	return ids, nil
}

// probeBalance reports one provider's balance through its balance probe, with
// the fix of a probe that cannot run or fails. A provider without a probe, or
// whose probe this build lacks, is reported as such and has no fix.
func probeBalance(ctx context.Context, runtime invocation.Context, cfg domain.Config, id string) (balanceItem, string) {
	provider := cfg.Providers[id]
	provider.ID = id
	item := balanceItem{ID: id, Label: provider.Label, State: balanceNoProbe}
	switch {
	case provider.BalanceProbe == nil:
		item.Detail = "No balance probe"
		return item, ""
	case !balanceprobe.Supports(provider.BalanceProbe.Kind):
		item.Detail = "This AIGW build has no " + provider.BalanceProbe.Kind + " balance probe"
		return item, ""
	}
	item.State = balanceFailed
	credential, err := runtime.Providers.Get(id)
	if err != nil {
		item.Problem = "the balance credentials cannot be read: " + credentialReason(err)
		return item, balanceCredentialFix(runtime.Secrets, id)
	}
	token, err := runtime.Secrets.Get(id)
	if err != nil {
		item.Problem = "the Token cannot be read: " + credentialReason(err)
		return item, tokenFix(runtime.Secrets, id, err)
	}
	ctx, cancel := context.WithTimeout(ctx, balanceTimeout)
	defer cancel()
	report, err := balanceprobe.Probe(ctx, probe.Sender{Client: runtime.HTTP}, provider, token, diagnostic.Credential(credential))
	if err != nil {
		item.Problem = err.Error()
		return item, "aigw provider balance " + id
	}
	item.State, item.Balance = balanceReported, &report.AccountBalance
	item.Token = &balanceToken{
		Name: report.TokenName, State: report.TokenStatus, Used: report.TokenUsed, Remaining: report.TokenRemaining,
		UnlimitedQuota: report.TokenUnlimitedQuota, RemainingRequests: report.TokenRemainingCount, UnlimitedRequests: report.TokenUnlimitedCount,
	}
	return item, ""
}

// credentialReason says why a credential cannot be read, without the
// backend's own message: it is missing, or the backend failed, which does not
// mean that it is missing.
func credentialReason(err error) string {
	if errors.Is(err, secret.ErrNotFound) {
		return "none is stored"
	}
	return "the credential backend failed, which does not mean that none is stored"
}

// balanceCredentialFix stores the balance credentials again, or with the
// read-only environment backend sets the variables that supply them.
func balanceCredentialFix(store secret.Store, id string) string {
	if variables := balanceVariables(store, id); len(variables) > 0 {
		return "Set " + variables[0] + " and " + variables[1] + " in aigw's environment."
	}
	return "aigw provider edit " + id + " --balance-token-stdin --balance-user-id <user-id>"
}

// tokenFix stores a missing Token, or sets the variable that supplies it; a
// backend that failed is a matter for `aigw check`.
func tokenFix(store secret.Store, id string, err error) string {
	switch {
	case !errors.Is(err, secret.ErrNotFound):
		return "aigw check"
	case secret.IsReadOnly(store):
		return "Set " + secret.EnvironmentKey(id) + " in aigw's environment."
	default:
		return "aigw provider rotate " + id
	}
}

func renderBalance(runtime invocation.Context, document balanceDocument, next string) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Balances")
	for _, item := range document.Providers {
		r.Section(item.Label)
		switch item.State {
		case balanceReported:
			r.Rows(balanceRows(item)...)
		case balanceNoProbe:
			r.Rows(terminal.Field{State: terminal.Info, Label: "Balance", Value: item.Detail})
		default:
			r.Rows(terminal.Field{State: terminal.Fail, Label: "Balance", Value: item.Problem, Details: []string{"Fix: " + item.NextAction + item.NextDetail}})
		}
	}
	r.Next(next)
	return r.Err()
}

// balanceRows are a reported balance and the quota of the provider's Token.
func balanceRows(item balanceItem) []terminal.Field {
	token := item.Token
	state, status := terminal.OK, "Enabled"
	if token.State != "enabled" {
		state, status = terminal.Fail, "Disabled"
	}
	remaining := fmt.Sprintf("$%.4f", token.Remaining)
	if token.UnlimitedQuota {
		remaining = "Unlimited"
	}
	requests := fmt.Sprintf("%d requests", token.RemainingRequests)
	if token.UnlimitedRequests {
		requests = "Unlimited requests"
	}
	return []terminal.Field{
		{Label: "Balance", Value: fmt.Sprintf("$%.4f", *item.Balance)},
		{Label: "Token", Value: token.Name},
		{State: state, Label: "Token status", Value: status},
		{Label: "Quota used", Value: fmt.Sprintf("$%.4f", token.Used)},
		{Label: "Remaining quota", Value: remaining},
		{Label: "Remaining requests", Value: requests},
	}
}
