package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// addOptions are the flags of `aigw model add`.
type addOptions struct {
	protocol     domain.EndpointProtocol
	family       string
	capabilities []string
	label        string
	unlisted     bool
	dryRun       bool
	jsonMode     bool
}

func newAddCommand(runtime invocation.Context) *cobra.Command {
	var options addOptions
	cmd := &cobra.Command{
		Use:   "add <provider/model>",
		Short: "Add a model that a provider serves; selects it for no client",
		Long: "Add a model that a provider serves; selects it for no client.\n" +
			"AIGW first reads the provider's catalog and adds only a model that it lists; --unlisted skips that check.",
		Example: "aigw model add dmxapi/gpt-6.2\n" +
			"aigw model add ucloud/minimax-m3 --protocol anthropic --capability tool_search\n" +
			"aigw model add dmxapi/gpt-6.1-sol-cdx --family gpt-6.1-sol --dry-run",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := modelArgument(cmd, args); err != nil {
				return err
			}
			if cmd.Flags().Changed("family") && !domain.ValidIdentifier(options.family) {
				return fmt.Errorf("--family %q is not a family ID; use letters, numbers, dots, hyphens or underscores; run `%s --help`", options.family, cmd.CommandPath())
			}
			if cmd.Flags().Changed("label") && strings.TrimSpace(options.label) == "" {
				return fmt.Errorf("--label requires a non-empty value; run `%s --help`", cmd.CommandPath())
			}
			_, err := parseCapabilities(cmd, options.capabilities)
			return err
		},
		ValidArgsFunction: invocation.CompleteNothing,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, runtime, args, options)
		},
	}
	invocation.BindProtocol(cmd, &options.protocol, "The protocol on which AIGW may use the model; optional when the provider has one endpoint")
	cmd.Flags().StringVar(&options.family, "family", "", "The model's family; an unknown family is created")
	cmd.Flags().StringArrayVar(&options.capabilities, "capability", nil, "A qualified `capability`; repeatable: "+capabilityUsage())
	cmd.Flags().StringVar(&options.label, "label", "", "Display name")
	cmd.Flags().BoolVar(&options.unlisted, "unlisted", false, "Add the model without reading the provider's catalog")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the addition without reading the catalog or writing anything")
	cmd.Flags().BoolVar(&options.jsonMode, "json", false, "Write the added model as JSON")
	invocation.CompleteFlag(cmd, "family", invocation.CompleteFamilies(runtime))
	invocation.CompleteFlag(cmd, "capability", invocation.CompleteCapabilities)
	invocation.CompleteFlag(cmd, "label", cobra.NoFileCompletions)
	return cmd
}

func runAdd(cmd *cobra.Command, runtime invocation.Context, args []string, options addOptions) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	ref, err := invocation.ParseModelArgument(cfg, args[0])
	if err != nil {
		return err
	}
	provider, err := invocation.ResolveProvider(cfg, ref.Provider())
	if err != nil {
		return err
	}
	if len(cfg.ResolveModelRef(ref)) > 0 {
		return terminal.ProblemError(ref.String()+" is already added; `aigw model edit` changes it", "", "", "aigw model edit "+ref.String()+" --help", nil)
	}
	protocol, err := addProtocol(cmd, ref, provider, options.protocol)
	if err != nil {
		return err
	}
	capabilities, _ := parseCapabilities(cmd, options.capabilities)
	declared, unadmitted := declare(ref, []domain.EndpointProtocol{protocol}, capabilities)
	if unadmitted != nil {
		return capabilityRefusal(cmd, provider, unadmitted)
	}
	before := cfg.Clone()
	model := domain.Model{Label: strings.TrimSpace(options.label), Provider: ref.Provider(), UpstreamModel: ref.ID(), Protocols: declared}
	family, created := familyFor(&cfg, ref, options.family)
	model.Family = family
	cfg.Models[domain.ModelKey(ref)] = model
	// A preview and --unlisted read no catalog.
	checked := !options.dryRun && !options.unlisted
	if checked {
		if err := checkCatalog(cmd.Context(), runtime, cmd, before, ref); err != nil {
			return err
		}
	}
	item := describe(cfg, domain.ModelKey(ref))
	rows := append(modelFields(runtime, item), catalogField(options))
	if created != "" {
		rows = append(rows, terminal.Field{Label: "New family", Value: created})
	}
	next := "aigw use <client> " + ref.String()
	return change{
		before: before, after: cfg, subject: "model add", version: addSchemaVersion, title: "Model added", rows: rows, next: next,
		document: changeDocument{modelItem: item, FamilyCreated: created, CatalogChecked: &checked},
	}.apply(cmd, runtime, args, options.dryRun, options.jsonMode)
}

// catalogField says whether the provider's catalog was checked.
func catalogField(options addOptions) terminal.Field {
	switch {
	case options.dryRun:
		return terminal.Field{State: terminal.Info, Label: "Catalog", Value: "Not checked in a preview"}
	case options.unlisted:
		return terminal.Field{State: terminal.Warn, Label: "Catalog", Value: "Not checked: --unlisted"}
	default:
		return terminal.Field{State: terminal.OK, Label: "Catalog", Value: "Listed by the provider"}
	}
}

// addProtocol is the protocol of a model that `aigw model add` adds: the one
// that --protocol names, which the provider must serve, or the provider's
// only endpoint.
func addProtocol(cmd *cobra.Command, ref domain.ModelRef, provider domain.Provider, requested domain.EndpointProtocol) (domain.EndpointProtocol, error) {
	served := providerProtocols(provider)
	if requested != "" {
		if !slices.Contains(served, requested) {
			return "", missingEndpoint(cmd, ref, provider, requested, protocolChoice(served))
		}
		return requested, nil
	}
	if len(served) == 1 {
		return served[0], nil
	}
	fix := invocation.Rerun(cmd, cmd.Flags().Args(), nil, "--protocol", protocolChoice(served))
	if fix == "" {
		fix = "Run the same command with --protocol " + protocolChoice(served) + "."
	}
	return "", terminal.ProblemError(
		fmt.Sprintf("provider %s serves %s, so %s needs --protocol", provider.ID, protocolLabels(served), ref), "", "", fix, nil)
}

// protocolChoice is a protocol to type: the one name, or a placeholder of the
// choice among several, which makes the fix a next_detail.
func protocolChoice(protocols []domain.EndpointProtocol) string {
	names := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		names = append(names, protocol.Name())
	}
	if len(names) == 1 {
		return names[0]
	}
	return "<" + strings.Join(names, "|") + ">"
}

// providerProtocols returns the protocols for which provider has an endpoint,
// in display order.
func providerProtocols(provider domain.Provider) []domain.EndpointProtocol {
	var protocols []domain.EndpointProtocol
	for _, name := range domain.ProtocolNames() {
		if protocol, _ := domain.ProtocolNamed(name); provider.Endpoints.For(protocol) != "" {
			protocols = append(protocols, protocol)
		}
	}
	return protocols
}

// missingEndpoint refuses a protocol for which the provider has no endpoint,
// naming the model, the provider and the missing protocol. Its fix is the
// command rerun with the protocols that would do: one that the provider
// serves for add, and the given ones that it serves for edit.
func missingEndpoint(cmd *cobra.Command, ref domain.ModelRef, provider domain.Provider, protocol domain.EndpointProtocol, protocols ...string) error {
	served := providerProtocols(provider)
	words := make([]string, 0, 2*len(protocols))
	for _, choice := range protocols {
		words = append(words, "--protocol", choice)
	}
	fix := invocation.Rerun(cmd, cmd.Flags().Args(), []string{"protocol"}, words...)
	if fix == "" || len(protocols) == 0 {
		fix = "Run the same command with --protocol " + protocolChoice(served) + "."
	}
	return terminal.ProblemError(
		fmt.Sprintf("%s cannot declare %s: provider %s has no %s endpoint", ref, domain.ProtocolLabel(protocol), provider.ID, domain.ProtocolLabel(protocol)),
		"Provider "+provider.ID+" serves "+protocolLabels(served)+".", "", fix, nil)
}

// familyFor is the family of a model that add creates: an explicit family,
// created labelled from the model when it does not exist, or else an existing
// family whose ID equals the model ID ignoring case, or else the model's own
// family. It returns the family that it creates, if any.
func familyFor(cfg *domain.Config, ref domain.ModelRef, explicit string) (family, created string) {
	if explicit != "" {
		if _, exists := cfg.Families[explicit]; !exists {
			cfg.Families[explicit] = domain.Family{Label: ref.ID()}
			return explicit, explicit
		}
		return explicit, ""
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Families)) {
		if strings.EqualFold(id, ref.ID()) {
			return id, ""
		}
	}
	family = domain.FamilyID(ref.ID())
	if _, exists := cfg.Families[family]; !exists {
		cfg.Families[family] = domain.Family{Label: ref.ID()}
	}
	return family, ""
}
