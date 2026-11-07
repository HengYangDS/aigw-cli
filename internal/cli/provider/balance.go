package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
		problem, fix := balanceCredentialProblem(runtime.Secrets, id, err)
		item.Problem = problem
		return item, fix
	}
	token, err := runtime.Secrets.Get(id)
	if err != nil {
		problem, fix := tokenProblem(runtime.Secrets, id, err)
		item.Problem = problem
		return item, fix
	}
	ctx, cancel := context.WithTimeout(ctx, balanceTimeout)
	defer cancel()
	report, err := balanceprobe.Probe(ctx, probe.Sender{Client: runtime.HTTP}, provider, token, diagnostic.Credential(credential))
	if err != nil {
		item.Problem = err.Error()
		return item, probeFix(runtime.Secrets, id, err)
	}
	item.State, item.Balance = balanceReported, &report.AccountBalance
	item.Token = &balanceToken{
		Name: report.TokenName, State: report.TokenStatus, Used: report.TokenUsed, Remaining: report.TokenRemaining,
		UnlimitedQuota: report.TokenUnlimitedQuota, RemainingRequests: report.TokenRemainingCount, UnlimitedRequests: report.TokenUnlimitedCount,
	}
	return item, ""
}

// balanceCredentialProblem says why the balance credentials cannot be read,
// and what fixes it. None stored: store them, or with the read-only
// environment backend set the variables that are not set, which it checks one
// by one, since one of the two alone supplies nothing. Any other failure is
// the backend's, which the provider's credential report shows.
func balanceCredentialProblem(store secret.Store, id string, err error) (string, string) {
	if variables := balanceVariables(store, id); len(variables) > 0 {
		set := secret.EnvironmentVariables(store, id)
		missing := slices.DeleteFunc(slices.Clone(variables), func(key string) bool { return slices.Contains(set, key) })
		if len(missing) == 1 {
			return "the balance credentials are incomplete: " + missing[0] + " is not set, and the other balance variable alone supplies nothing",
				"Set " + missing[0] + " in aigw's environment."
		}
		return "no balance credentials: " + variables[0] + " and " + variables[1] + " are not set", "Set " + variables[0] + " and " + variables[1] + " in aigw's environment."
	}
	if errors.Is(err, secret.ErrNotFound) {
		return "no balance credentials are stored", "aigw provider edit " + id + " --balance-token-stdin --balance-user-id <user-id>"
	}
	return "the credential backend could not read the balance credentials", "aigw provider show " + id
}

// tokenProblem says why the provider's Token cannot be read, and what fixes
// it: store a missing Token, or set the variable that supplies it; a backend
// that failed is shown by the provider's credential report.
func tokenProblem(store secret.Store, id string, err error) (string, string) {
	switch {
	case !errors.Is(err, secret.ErrNotFound):
		return "the credential backend could not read the Token", "aigw provider show " + id
	case secret.IsReadOnly(store):
		return "no Token: " + secret.EnvironmentKey(id) + " is not set", "Set " + secret.EnvironmentKey(id) + " in aigw's environment."
	default:
		return "no Token is stored", "aigw provider rotate " + id
	}
}

// probeFix is the fix of a balance probe that failed. A request without a
// response and a status that a later request may not repeat need the same
// probe again. A refusal of the balance credentials, as HTTP 401 or 403 or a
// refused query, needs new ones; an account that lists no matching Token
// needs the provider's Token replaced; an account with more Tokens than the
// probe's pages hold needs fewer, which no setting changes. Any other
// failure, such as another status or a response that is not the platform's,
// points at the probe's settings in the configuration file. With the
// read-only environment backend the fix sets the variables and then runs the
// probe.
func probeFix(store secret.Store, id string, err error) string {
	status, answered := errors.AsType[*diagnostic.StatusError](err)
	_, unreachable := errors.AsType[*diagnostic.UnreachableError](err)
	_, refused := errors.AsType[*diagnostic.RefusedError](err)
	rerun := "aigw provider balance " + id
	readOnly := secret.IsReadOnly(store)
	switch {
	case unreachable, answered && invocation.TransientStatus(status.Status):
		return rerun
	case refused, answered && (status.Status == http.StatusUnauthorized || status.Status == http.StatusForbidden):
		if variables := balanceVariables(store, id); readOnly {
			return "Set " + variables[0] + " and " + variables[1] + " to balance credentials that the provider accepts, then run `" + rerun + "`."
		}
		return "aigw provider edit " + id + " --balance-token-stdin --balance-user-id <user-id>"
	case errors.Is(err, diagnostic.ErrTokenNotListed) && readOnly:
		return "Set " + secret.EnvironmentKey(id) + " to the Token that the provider's account lists, then run `" + rerun + "`."
	case errors.Is(err, diagnostic.ErrTokenNotListed):
		return "aigw provider rotate " + id
	case errors.Is(err, diagnostic.ErrSearchIncomplete):
		return "The provider's account lists more Tokens than the balance probe reads; delete the Tokens that you no longer use at the provider, then run `" + rerun + "`."
	default:
		return "Correct the balance probe of provider " + id + " in the configuration file; `aigw config path` prints its location."
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
