package invocation

import (
	"maps"
	"slices"

	"aigw-cli/internal/domain"

	"github.com/spf13/cobra"
)

// The completion functions read only the local configuration: no Token, no
// credential metadata and no request, so a locked or failing credential store
// cannot slow or break them. A configuration that does not load completes
// nothing. Every function turns off the shell's file completion, because no
// argument that they complete names a file, and so does the completion of a
// new name or a free-text query, which offers nothing.

// CompleteNothing offers nothing and no file name: the completion of a new
// name or a free-text query, which the user types.
func CompleteNothing(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// CompleteClients completes a client ID from the admitted clients.
func CompleteClients(runtime Context) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		completions := []cobra.Completion{}
		for _, spec := range runtime.Clients.Admission().Specs() {
			completions = append(completions, cobra.CompletionWithDesc(spec.ID, spec.Label))
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteProviders completes a provider ID from the configuration.
func CompleteProviders(runtime Context) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		completions := []cobra.Completion{}
		for _, id := range cfg.CatalogProviderIDs() {
			completions = append(completions, cobra.CompletionWithDesc(id, cfg.Providers[id].Label))
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteModels completes the one `provider/model` argument of a command that
// names no client, from the added models, each name once, and nothing once it
// is given.
func CompleteModels(runtime Context) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return modelCompletions(runtime, "")
	}
}

// completeClientModels completes `<client> <provider/model>`, the arguments
// of `aigw use`: a client ID first, and then only the models that the client
// can use, those whose provider serves a protocol that the client speaks,
// which `aigw use` gives a model that lacks it. It offers nothing for an
// unknown client or once both are given. Task 4.1, which gives `aigw use`
// these arguments, binds it.
func completeClientModels(runtime Context) cobra.CompletionFunc {
	clients := CompleteClients(runtime)
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return clients(cmd, args, toComplete)
		case 1:
			if !runtime.Clients.Admission().Admits(args[0]) {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return modelCompletions(runtime, args[0])
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// modelCompletions offers each added model's name once, with its label, in
// display order, and only those that client can use when client names one.
func modelCompletions(runtime Context, client string) ([]cobra.Completion, cobra.ShellCompDirective) {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	spec, admitted := cfg.Admission().Spec(client)
	completions := []cobra.Completion{}
	offered := map[string]bool{}
	for _, key := range cfg.CatalogModelIDs() {
		model := cfg.Models[key]
		name := model.Ref().String()
		if offered[name] || admitted && len(spec.CompatibleProtocols(cfg.Providers[model.Provider])) == 0 {
			continue
		}
		offered[name] = true
		completions = append(completions, cobra.CompletionWithDesc(name, cfg.ModelLabel(key)))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// CompleteFamilies completes a family ID from the configuration's families.
func CompleteFamilies(runtime Context) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		completions := []cobra.Completion{}
		for _, id := range slices.Sorted(maps.Keys(cfg.Families)) {
			completions = append(completions, cobra.CompletionWithDesc(id, cfg.Families[id].Label))
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteCapabilities completes a capability from the fixed list that AIGW
// defines.
func CompleteCapabilities(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	completions := []cobra.Completion{}
	for _, capability := range domain.Capabilities() {
		completions = append(completions, string(capability))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// depthNames are the evidence depths of `aigw check`, in the order in which
// each runs the ones before it.
var depthNames = []string{"local", "endpoint", "model", "client"}

// completeDepths completes an evidence depth from the fixed list. Task 4.2
// gives `aigw check` the --depth flag that uses it.
func completeDepths(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return slices.Clone(depthNames), cobra.ShellCompDirectiveNoFileComp
}
