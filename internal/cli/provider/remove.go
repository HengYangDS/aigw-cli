package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// The states of a removal.
const (
	removalDone    = "removed"
	removalPlanned = "planned"
	removalBlocked = "blocked"
)

func newRemoveCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{
		Use:   "remove <provider>",
		Short: "Remove a provider with its models, Token and balance credentials",
		Long: "Remove a provider with its models, Token and balance credentials.\n" +
			"It refuses while a client selects one of the provider's models, and drops the provider's models from recommendations.\n" +
			"An ID that no provider has loses what the credential store still holds for it, as `aigw check` lists it.",
		Example:           "aigw provider remove ucloud --dry-run\naigw provider remove ucloud --json",
		Args:              providerArgument,
		ValidArgsFunction: completeProvider(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd, runtime, args, dryRun, jsonMode)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the removal without writing anything")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the removal as JSON")
	return cmd
}

func runRemove(cmd *cobra.Command, runtime invocation.Context, args []string, dryRun, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if _, configured := cfg.Providers[args[0]]; !configured {
		if err := refuseCaseVariant(cfg, args[0]); err != nil {
			return err
		}
		if removed, err := removeLeftover(cmd, runtime, args, dryRun, jsonMode); removed || err != nil {
			return err
		}
	}
	provider, err := invocation.ResolveProvider(cfg, args[0])
	if err != nil {
		return err
	}
	id := provider.ID
	document := removeDocument{
		DryRun: dryRun, Status: removalDone, ID: id, Label: provider.Label, Models: providerModels(cfg, id),
		Token: credentialState(runtime.Secrets.Exists, id), BalanceCredential: credentialState(runtime.Providers.Exists, id),
		Recommendations: []string{}, ClientFiles: []invocation.ClientFile{},
	}
	if secret.IsReadOnly(runtime.Secrets) {
		document.Unset = secret.EnvironmentVariables(runtime.Secrets, id)
	}
	if refusal := removalRefusal(runtime, cfg, id); refusal != nil {
		return blockedRemoval(runtime, document, refusal, jsonMode)
	}
	before := cfg.Clone()
	document.Recommendations = removeProvider(&cfg, id)
	preview, err := invocation.PreviewChange(runtime, before, cfg)
	if err != nil {
		return err
	}
	document.ClientFiles, document.CredentialReader = invocation.ClientFiles(runtime, preview), preview.CredentialEntrypoint
	next := "aigw provider list"
	if dryRun {
		document.Status, next = removalPlanned, invocation.ApplyCommand(cmd, args)
	} else if err := commitRemoval(cmd, runtime, before, cfg, id, document); err != nil {
		return err
	}
	if jsonMode {
		document.NextAction, document.NextDetail = terminal.NextSteps(next)
		return terminal.WriteDocument(runtime.Out, removeSchemaVersion, true, document)
	}
	return renderRemoval(runtime, document, next)
}

// refuseCaseVariant refuses an ID that differs only in case from a configured
// provider, before any store access. A file system that ignores case keeps
// both in one slot, so deleting the ID's credentials would delete the
// provider's. The fix previews the provider's removal, which changes nothing
// if it is run as it stands.
func refuseCaseVariant(cfg domain.Config, id string) error {
	for _, configured := range cfg.CatalogProviderIDs() {
		if domain.SameIdentifier(configured, id) {
			return terminal.ProblemError(fmt.Sprintf("%q differs only in case from provider %q, so AIGW does not remove it as a leftover", id, configured),
				"", "Nothing was deleted.", "aigw provider remove "+configured+" --dry-run", nil)
		}
	}
	return nil
}

// removeLeftover removes what a store holds for an ID that no provider has,
// the stored Tokens that `aigw check` lists as unused: there is no provider,
// model or client file to change, so no later step can fail and need the value
// back, and no value is read. It reports false, leaving the refusal of an
// unknown provider to its caller, when the store holds nothing for the ID or
// cannot tell. The read-only environment backend keeps its variables, which
// the result names.
func removeLeftover(cmd *cobra.Command, runtime invocation.Context, args []string, dryRun, jsonMode bool) (bool, error) {
	id := args[0]
	document := removeDocument{
		DryRun: dryRun, Status: removalDone, ID: id, Models: []string{},
		Token: credentialState(runtime.Secrets.Exists, id), BalanceCredential: credentialState(runtime.Providers.Exists, id),
		Recommendations: []string{}, ClientFiles: []invocation.ClientFile{},
	}
	if document.Token != credentialPresent && document.BalanceCredential != credentialPresent {
		return false, nil
	}
	readOnly := secret.IsReadOnly(runtime.Secrets)
	next := "aigw provider list"
	switch {
	case dryRun:
		document.Status, next = removalPlanned, invocation.ApplyCommand(cmd, args)
	case !readOnly:
		if err := deleteLeftover(cmd, runtime, args, document); err != nil {
			return false, err
		}
	}
	if readOnly {
		document.Unset = secret.EnvironmentVariables(runtime.Secrets, id)
		if !dryRun {
			next = "Unset " + strings.Join(document.Unset, " and ") + " in aigw's environment."
		}
	}
	if jsonMode {
		document.NextAction, document.NextDetail = terminal.NextSteps(next)
		return true, terminal.WriteDocument(runtime.Out, removeSchemaVersion, true, document)
	}
	return true, renderLeftover(runtime, document, next, readOnly)
}

// deleteLeftover deletes the slots that the document reports present, without
// reading them. One that cannot be deleted refuses, naming what was deleted
// before it, and the same removal converges.
func deleteLeftover(cmd *cobra.Command, runtime invocation.Context, args []string, document removeDocument) error {
	impact := "Nothing was deleted."
	if document.BalanceCredential != credentialAbsent {
		if err := runtime.Providers.Delete(document.ID); err != nil && !errors.Is(err, secret.ErrNotFound) {
			return terminal.ProblemError("the balance credentials stored for "+document.ID+" could not be deleted", "", impact, invocation.Retry(cmd, args), err)
		}
		impact = "The balance credentials were deleted; the Token stays stored."
	}
	if document.Token != credentialAbsent {
		if err := runtime.Secrets.Delete(document.ID); err != nil && !errors.Is(err, secret.ErrNotFound) {
			return terminal.ProblemError("the Token stored for "+document.ID+" could not be deleted", "", impact, invocation.Retry(cmd, args), err)
		}
	}
	return nil
}

// renderLeftover reports the removal of what a store held for an ID that no
// provider has.
func renderLeftover(runtime invocation.Context, document removeDocument, next string, readOnly bool) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Stored credentials without a provider")
	tokenVariable := slices.DeleteFunc(slices.Clone(document.Unset), func(key string) bool { return key != secret.EnvironmentKey(document.ID) })
	balanceVariables := slices.DeleteFunc(slices.Clone(document.Unset), func(key string) bool { return key == secret.EnvironmentKey(document.ID) })
	r.Rows(terminal.Field{Label: "Provider", Value: document.ID + " (not configured)"},
		removedCredential("Token", document.Token, tokenVariable, "Deleted"), removedCredential("Balance token", document.BalanceCredential, balanceVariables, "Deleted"))
	switch {
	case readOnly:
		r.Success("Nothing was deleted: the environment supplies these until you unset them")
	case document.DryRun:
		r.Success("Nothing was written: the credential store is unchanged")
	default:
		r.Success("Removed; no provider, model or client changed")
	}
	r.Next(next)
	return r.Err()
}

// removalRefusal refuses a removal that would leave a client without its
// model, enabled or disabled: the fix moves the client. The last provider and
// the last model may go, which leaves a valid configuration whose clients
// select nothing.
func removalRefusal(runtime invocation.Context, cfg domain.Config, id string) error {
	if clients := selectingClients(cfg, id); len(clients) > 0 {
		models := []string{}
		for _, client := range clients {
			if model := cfg.Models[cfg.SelectedModel(client)].Ref().String(); !slices.Contains(models, model) {
				models = append(models, model)
			}
		}
		verb := "uses"
		if len(clients) > 1 {
			verb = "use"
		}
		return terminal.ProblemError(fmt.Sprintf("%s %s %s, so provider %s cannot be removed",
			clientLabels(runtime.Clients.Admission(), clients), verb, strings.Join(models, ", "), id), "", "", "aigw use "+clients[0]+" <provider/model>", nil)
	}
	return nil
}

// blockedRemoval reports a refused removal. A JSON preview writes its own
// document with the blocked status, ok false and the fix; any other run fails
// with the refusal.
func blockedRemoval(runtime invocation.Context, document removeDocument, refusal error, jsonMode bool) error {
	if !document.DryRun || !jsonMode {
		return refusal
	}
	problem := terminal.ProblemFor(refusal)
	document.Status, document.Problem = removalBlocked, problem.Title
	document.NextAction, document.NextDetail = terminal.NextSteps(problem.Fix)
	if err := terminal.WriteDocument(runtime.Out, removeSchemaVersion, false, document); err != nil {
		return err
	}
	return terminal.Presented(refusal)
}

// removeProvider removes the provider and its models from cfg, dropping them
// from recommendations, and returns the clients whose recommendation changed.
func removeProvider(cfg *domain.Config, id string) []string {
	changed := []string{}
	for _, key := range cfg.ModelIDs() {
		if cfg.Models[key].Provider != id {
			continue
		}
		delete(cfg.Models, key)
		for _, client := range cfg.RemoveRecommendedModel(key) {
			if !slices.Contains(changed, client) {
				changed = append(changed, client)
			}
		}
	}
	delete(cfg.Providers, id)
	slices.Sort(changed)
	return changed
}

// commitRemoval deletes the provider's credentials and saves the configuration
// without it, as one transaction: a save that fails puts the credentials back,
// and when one cannot be put back the failure names it, with the removal
// again as the fix, which converges. The read-only environment backend keeps
// its variables, which the result names.
func commitRemoval(cmd *cobra.Command, runtime invocation.Context, before, after domain.Config, id string, document removeDocument) error {
	var changes []credentialChange
	if !secret.IsReadOnly(runtime.Secrets) {
		var err error
		if changes, err = deleteCredentials(runtime, id, document); err != nil {
			return err
		}
	}
	if err := runtime.Synchronizer().Commit(cmd.Context(), before, after, "provider remove"); err != nil {
		return saveFailure(err, changes, "provider "+id+" was not removed", "aigw provider remove "+id)
	}
	return nil
}

func renderRemoval(runtime invocation.Context, document removeDocument, next string) error {
	r := invocation.Renderer(runtime)
	title := "Provider removed"
	if document.DryRun {
		title += " (preview)"
	}
	r.ProductTitle(title)
	models := strings.Join(document.Models, ", ")
	if models == "" {
		models = "None"
	}
	rows := []terminal.Field{{Label: "Provider", Value: document.ID}, {Label: "Label", Value: document.Label}, {Label: "Models", Value: models}}
	tokenVariable := slices.DeleteFunc(slices.Clone(document.Unset), func(key string) bool { return key != secret.EnvironmentKey(document.ID) })
	balanceVariables := slices.DeleteFunc(slices.Clone(document.Unset), func(key string) bool { return key == secret.EnvironmentKey(document.ID) })
	rows = append(rows, removedCredential("Token", document.Token, tokenVariable, deletedWithProvider), removedCredential("Balance token", document.BalanceCredential, balanceVariables, deletedWithProvider))
	if len(document.Recommendations) > 0 {
		rows = append(rows, terminal.Field{Label: "Recommendations", Value: "Dropped for " + clientLabels(runtime.Clients.Admission(), document.Recommendations)})
	}
	r.Rows(rows...)
	invocation.RenderClientFiles(r, runtime, document.ClientFiles)
	if document.DryRun {
		r.Success("Nothing was written: configuration, credentials and client files are unchanged")
	} else {
		r.Success("Removed; no other provider changed")
	}
	r.Next(next)
	return r.Err()
}

// deletedWithProvider is the row of a credential that a provider's removal
// deletes.
const deletedWithProvider = "Deleted with the provider"

// removedCredential is the row of a credential that a removal deletes: one
// that is stored, none, one that the backend could not observe, or one whose
// variables the read-only environment backend still sees set, each checked
// on its own, which stay until they are unset.
func removedCredential(label, state string, variables []string, deleted string) terminal.Field {
	switch {
	case len(variables) > 0:
		return terminal.Field{State: terminal.Warn, Label: label, Value: "Still set in the environment; unset " + strings.Join(variables, " and ")}
	case state == credentialPresent:
		return terminal.Field{State: terminal.OK, Label: label, Value: deleted}
	case state == credentialUnobserved:
		return terminal.Field{State: terminal.Warn, Label: label, Value: "Unknown: the credential backend could not tell; deleted if stored"}
	default:
		return terminal.Field{Label: label, Value: "None stored"}
	}
}
