package model

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// The states of a model that search reports: added and listed, listed and not
// added, added and no longer listed by any source of a complete read, which
// outranks deprecated, and added, deprecated and listed.
const (
	stateAdded      = "added"
	stateAvailable  = "available"
	stateMissing    = "missing"
	stateDeprecated = "deprecated"
)

func newSearchCommand(runtime invocation.Context) *cobra.Command {
	var providers []string
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search what providers offer, from their authenticated catalogs",
		Long: "Search what providers offer, from their authenticated catalogs.\n" +
			"Each model is added, available, missing (added, but no longer listed) or deprecated. " +
			"A catalog does not say which client can use a model.",
		Example: "aigw model search\n" +
			"aigw model search sonnet --provider dmxapi --provider ucloud\n" +
			"aigw model search gpt 6.1 --json",
		ValidArgsFunction: invocation.CompleteNothing,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, runtime, args, providers, jsonMode)
		},
	}
	cmd.Flags().StringSliceVar(&providers, "provider", nil, "Only this provider's catalog; repeatable")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the models as JSON")
	invocation.CompleteFlag(cmd, "provider", invocation.CompleteProviders(runtime))
	return cmd
}

func runSearch(cmd *cobra.Command, runtime invocation.Context, words, providers []string, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	ids, err := searchedProviders(cfg, providers)
	if err != nil {
		return err
	}
	scoped := cfg.Clone()
	for id := range scoped.Providers {
		if !slices.Contains(ids, id) {
			delete(scoped.Providers, id)
		}
	}
	report := app.DiscoverCatalog(cmd.Context(), runtime.Secrets, runtime.HTTP, scoped)
	document := searchDocument{Providers: make([]searchProvider, 0, len(ids))}
	next := "aigw model add <provider/model>"
	failed := false
	for _, id := range ids {
		result := searchResult(cmd, runtime, cfg, id, report, words)
		if result.fix != "" && !failed {
			failed, next = true, result.fix
		}
		document.Providers = append(document.Providers, result)
	}
	if jsonMode {
		document.NextAction, document.NextDetail = terminal.NextSteps(next)
		if err := terminal.WriteDocument(runtime.Out, searchSchemaVersion, !failed, document); err != nil {
			return err
		}
	} else if err := renderSearch(runtime, document, next); err != nil {
		return err
	}
	if failed {
		return terminal.Presented(errors.New("a provider's catalog could not be read completely"))
	}
	return nil
}

// searchedProviders are the providers whose catalogs search reads, in
// configuration order: those that --provider names, or every one.
func searchedProviders(cfg domain.Config, names []string) ([]string, error) {
	if len(cfg.Providers) == 0 {
		return nil, terminal.ProblemError("no provider is configured", "", "", "aigw setup", nil)
	}
	for _, name := range names {
		if _, err := invocation.ResolveProvider(cfg, name); err != nil {
			return nil, err
		}
	}
	ids := cfg.CatalogProviderIDs()
	if len(names) > 0 {
		ids = slices.DeleteFunc(ids, func(id string) bool { return !slices.Contains(names, id) })
	}
	return ids, nil
}

// searchResult merges one provider's catalog observations into one row per
// model: the added models in configuration order, then the listed models that
// are not added, by ID, each with the protocol sources that listed it. A
// source that could not be read is named with its reason, and the result
// carries the fix of the first one; such a partial read reports no model as
// missing.
func searchResult(cmd *cobra.Command, runtime invocation.Context, cfg domain.Config, id string, report app.CatalogReport, words []string) searchProvider {
	provider := cfg.Providers[id]
	result := searchProvider{ID: id, Label: provider.Label, Sources: []searchSource{}, Models: []searchModel{}}
	listed := map[string][]string{}
	var reasons []string
	for _, observation := range report.Observations {
		if observation.Provider != id {
			continue
		}
		// A source names the endpoint as the provider configures it, which
		// `aigw provider edit` changes, whether or not it was read.
		endpoint := strings.TrimRight(provider.Endpoints.For(observation.Source.Protocol), "/")
		source := searchSource{Protocol: observation.Source.Protocol.Name(), Endpoint: endpoint, Status: string(observation.Status)}
		result.Sources = append(result.Sources, source)
		if observation.Status != app.CatalogOK {
			reason, fix := sourceProblem(cmd, runtime, id, observation)
			reasons = append(reasons, sourceLabel(observation.Source.Protocol)+": "+reason)
			result.fix = cmp.Or(result.fix, fix)
			continue
		}
		result.read = true
		for _, model := range observation.Models {
			listed[model.ID] = append(listed[model.ID], source.Protocol)
		}
	}
	added := map[string]bool{}
	for _, keys := range modelsByName(cfg) {
		item := describe(cfg, keys...)
		if item.Provider != id {
			continue
		}
		_, modelID, _ := strings.Cut(item.ID, "/")
		added[modelID] = true
		sources, isListed := listed[modelID]
		state := stateAdded
		switch {
		case !isListed && len(reasons) > 0:
			continue
		case !isListed:
			state = stateMissing
		case item.Deprecated:
			state = stateDeprecated
		}
		if matches(words, modelID, item.Label) {
			result.Models = append(result.Models, searchModel{ID: item.ID, State: state, Label: item.Label, ListedBy: append([]string{}, sources...)})
		}
	}
	for _, modelID := range slices.Sorted(maps.Keys(listed)) {
		if !added[modelID] && matches(words, modelID, "") {
			result.Models = append(result.Models, searchModel{ID: id + "/" + modelID, State: stateAvailable, ListedBy: listed[modelID]})
		}
	}
	result.Problem = strings.Join(reasons, "; ")
	result.NextAction, result.NextDetail = terminal.NextSteps(result.fix)
	return result
}

// sourceLabel names a protocol source by its protocol's label, or says that
// the provider has no endpoint to read.
func sourceLabel(protocol domain.EndpointProtocol) string {
	if protocol == "" {
		return "No endpoint"
	}
	return domain.ProtocolLabel(protocol)
}

// sourceProblem says why a protocol source could not be read, and what fixes
// it: a missing Token is stored or set; a credential backend that failed is
// shown by the provider's credential report; a rejected Token is replaced, or
// its variable changed before the same command runs again; a status that a
// later request may not repeat and a request that did not complete need the
// same command again; and any other status or a response that is not a
// catalog points at the endpoint.
func sourceProblem(cmd *cobra.Command, runtime invocation.Context, id string, observation app.CatalogObservation) (string, string) {
	key, readOnly := secret.EnvironmentKey(id), secret.IsReadOnly(runtime.Secrets)
	retry := invocation.Retry(cmd, cmd.Flags().Args())
	rotate := "aigw provider rotate " + id
	if readOnly {
		rotate = "Set " + key + " to a Token that the provider accepts, then run the same command again."
		if line := invocation.Rerun(cmd, cmd.Flags().Args(), nil); line != "" {
			rotate = "Set " + key + " to a Token that the provider accepts, then run `" + line + "`."
		}
	}
	edit := "aigw provider edit " + id + " --endpoint " + cmp.Or(observation.Source.Protocol.Name(), "<protocol>") + "=<url>"
	status, answered := errors.AsType[*app.CatalogStatusError](observation.Failure)
	switch {
	case observation.Status == app.CatalogTokenUnavailable && readOnly:
		return "no Token: " + key + " is not set", "Set " + key + " in aigw's environment."
	case observation.Status == app.CatalogTokenUnavailable:
		return "no Token is stored", "aigw provider rotate " + id
	case observation.Status == app.CatalogCredentialUnavailable:
		return "the credential backend could not tell whether a Token is stored", "aigw provider show " + id
	case observation.Status != app.CatalogRequestFailed:
		return "no endpoint", edit
	case answered && (status.Status == http.StatusUnauthorized || status.Status == http.StatusForbidden):
		return fmt.Sprintf("the provider rejected the Token (HTTP %d)", status.Status), rotate
	case answered && invocation.TransientStatus(status.Status):
		return fmt.Sprintf("HTTP %d", status.Status), retry
	case answered:
		return fmt.Sprintf("HTTP %d", status.Status), edit
	case errors.Is(observation.Failure, app.ErrCatalogFormat):
		return "the response is not a model catalog", edit
	default:
		return "the request did not complete", retry
	}
}

// matches reports whether every query word, ignoring case, appears in a
// model's ID or label.
func matches(words []string, id, label string) bool {
	text := strings.ToLower(id + "\x00" + label)
	for _, word := range words {
		for part := range strings.FieldsSeq(strings.ToLower(word)) {
			if !strings.Contains(text, part) {
				return false
			}
		}
	}
	return true
}

// searchStates are the symbols of the model states in human output.
var searchStates = map[string]terminal.State{
	stateAdded: terminal.OK, stateAvailable: terminal.Info, stateMissing: terminal.Warn, stateDeprecated: terminal.Warn,
}

func renderSearch(runtime invocation.Context, document searchDocument, next string) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Model search")
	models, providers := 0, 0
	for _, provider := range document.Providers {
		r.Section(provider.Label)
		rows := make([]terminal.Field, 0, len(provider.Models)+1)
		for _, model := range provider.Models {
			value := model.State
			if model.Label != "" {
				value += " · " + model.Label
			}
			rows = append(rows, terminal.Field{State: searchStates[model.State], Label: model.ID, Value: value})
		}
		if provider.Problem != "" {
			rows = append(rows, terminal.Field{State: terminal.Fail, Label: "Not read", Value: provider.Problem, Details: []string{"Fix: " + provider.fix}})
		} else if len(rows) == 0 {
			rows = append(rows, terminal.Field{State: terminal.Info, Label: "No model matches"})
		}
		r.Rows(rows...)
		models += len(provider.Models)
		if provider.read {
			providers++
		}
	}
	r.Section(quantity(models, "model") + " from " + quantity(providers, "provider"))
	r.Next(next)
	return r.Err()
}

// quantity states a number of things, such as "1 model" or "3 models".
func quantity(number int, thing string) string {
	if number == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", number, thing)
}
