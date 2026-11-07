package model

import (
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// editOptions are the flags of `aigw model edit`. A repeatable set that is
// given replaces the model's set.
type editOptions struct {
	protocols    []domain.EndpointProtocol
	family       string
	capabilities []string
	label        string
	dryRun       bool
	jsonMode     bool
}

func newEditCommand(runtime invocation.Context) *cobra.Command {
	var options editOptions
	cmd := &cobra.Command{
		Use:   "edit <provider/model>",
		Short: "Change a model's protocols, family, capabilities or label",
		Long: "Change a model's protocols, family, capabilities or label.\n" +
			"Each repeatable flag replaces the model's set; --capability \"\" clears the capabilities.",
		Example: "aigw model edit dmxapi/gpt-6.1-sol-cdx --family gpt-6.1-sol\n" +
			"aigw model edit ucloud/minimax-m3 --capability tool_search\n" +
			"aigw model edit ucloud/gpt-6.1-sol --protocol responses --protocol chat --dry-run",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := modelArgument(cmd, args); err != nil {
				return err
			}
			if !slices.ContainsFunc([]string{"protocol", "family", "capability", "label"}, cmd.Flags().Changed) {
				return fmt.Errorf("model edit needs --protocol, --family, --capability or --label; run `%s --help`", cmd.CommandPath())
			}
			if cmd.Flags().Changed("family") && !domain.ValidIdentifier(options.family) {
				return fmt.Errorf("--family %q is not a family ID; use letters, numbers, dots, hyphens or underscores; run `%s --help`", options.family, cmd.CommandPath())
			}
			_, err := parseCapabilities(cmd, options.capabilities)
			return err
		},
		ValidArgsFunction: invocation.CompleteModels(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(cmd, runtime, args, options)
		},
	}
	invocation.BindProtocols(cmd, &options.protocols, "Replace the model's protocols; repeatable")
	invocation.BindText(cmd, &options.family, "family", "The model's family; an unknown family is created")
	cmd.Flags().StringArrayVar(&options.capabilities, "capability", nil, "Replace the model's capabilities with this `capability`; repeatable, \"\" clears them: "+capabilityUsage())
	invocation.BindText(cmd, &options.label, "label", "Display name; \"\" clears it")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the edit without writing anything")
	cmd.Flags().BoolVar(&options.jsonMode, "json", false, "Write the edited model as JSON")
	invocation.CompleteFlag(cmd, "family", invocation.CompleteFamilies(runtime))
	invocation.CompleteFlag(cmd, "capability", invocation.CompleteCapabilities)
	invocation.CompleteFlag(cmd, "label", cobra.NoFileCompletions)
	return cmd
}

func runEdit(cmd *cobra.Command, runtime invocation.Context, args []string, options editOptions) error {
	// --protocol replaces the protocols, so it cannot also select among the
	// models of one name; such a name is refused with `aigw model show`.
	cfg, key, model, err := loadModel(runtime, args[0], modelSelector{})
	if err != nil {
		return err
	}
	before := cfg.Clone()
	ref := model.Ref()
	var pinned []string
	if cmd.Flags().Changed("protocol") {
		provider := cfg.Providers[model.Provider]
		provider.ID = model.Provider
		for _, protocol := range options.protocols {
			if provider.Endpoints.For(protocol) == "" {
				return missingEndpoint(cmd, ref, provider, protocol, servedNames(provider, options.protocols)...)
			}
		}
		if err := keepSelectedProtocols(cfg, key, ref, options.protocols); err != nil {
			return err
		}
		if pinned, err = pinRecommendations(&cfg, key, ref, options.protocols); err != nil {
			return err
		}
		pinSelections(&cfg, key)
		model.Protocols = replaceProtocols(model.Protocols, options.protocols)
	}
	if cmd.Flags().Changed("capability") {
		capabilities, _ := parseCapabilities(cmd, options.capabilities)
		declared, unadmitted := declare(ref, model.AdmittedProtocols(), capabilities)
		if unadmitted != nil {
			provider := cfg.Providers[model.Provider]
			provider.ID = model.Provider
			return capabilityRefusal(cmd, provider, unadmitted)
		}
		model.Protocols = declared
	}
	created := ""
	if cmd.Flags().Changed("family") {
		model.Family, created = familyFor(&cfg, ref, options.family)
	}
	if cmd.Flags().Changed("label") {
		model.Label = strings.TrimSpace(options.label)
	}
	cfg.Models[key] = model
	item := describe(cfg, key)
	rows := modelFields(runtime, item)
	if created != "" {
		rows = append(rows, terminal.Field{Label: "New family", Value: created})
	}
	if len(pinned) > 0 {
		rows = append(rows, terminal.Field{Label: "Recommendations", Value: "Kept on their current protocols for " + clientLabels(runtime.Clients.Admission(), pinned)})
	}
	return change{
		before: before, after: cfg, subject: "model edit", version: editSchemaVersion, title: "Model edited", rows: rows,
		next:     "aigw model show " + ref.String(),
		document: changeDocument{modelItem: item, FamilyCreated: created, PinnedRecommendations: pinned},
	}.apply(cmd, runtime, args, options.dryRun, options.jsonMode)
}

// keepSelectedProtocols refuses protocols that omit the protocol of a client
// that selects the model: `aigw use` moves the client to one of the protocols
// that remain first, after which the edit can run.
func keepSelectedProtocols(cfg domain.Config, key string, ref domain.ModelRef, protocols []domain.EndpointProtocol) error {
	for _, client := range cfg.SelectedClientsForModel(key) {
		selected, err := cfg.ResolveRuntime(client, "")
		if err != nil || slices.Contains(protocols, selected.Protocol) {
			continue
		}
		spec, _ := cfg.Admission().Spec(client)
		var remaining []string
		for _, protocol := range protocols {
			if slices.Contains(spec.EndpointProtocols, protocol) {
				remaining = append(remaining, protocol.Name())
			}
		}
		fix := fmt.Sprintf("aigw use %s <provider/model>", client)
		switch len(remaining) {
		case 0:
		case 1:
			fix = fmt.Sprintf("aigw use %s %s --protocol %s", client, ref, remaining[0])
		default:
			fix = fmt.Sprintf("aigw use %s %s --protocol <%s>", client, ref, strings.Join(remaining, "|"))
		}
		return terminal.ProblemError(
			fmt.Sprintf("%s uses %s on %s, which this edit would remove", spec.Label, ref, domain.ProtocolLabel(selected.Protocol)),
			"", "", fix, nil)
	}
	return nil
}

// servedNames returns the names of the protocols among given that provider
// serves, or the choice of the protocols that it serves when it serves none of
// them.
func servedNames(provider domain.Provider, given []domain.EndpointProtocol) []string {
	var names []string
	for _, protocol := range given {
		if provider.Endpoints.For(protocol) != "" {
			names = append(names, protocol.Name())
		}
	}
	if len(names) == 0 {
		// None of the given protocols is served: the fix gives one that is,
		// as add's does.
		return []string{protocolChoice(providerProtocols(provider))}
	}
	return names
}

// replaceProtocols gives a model exactly protocols: a protocol that it keeps
// keeps the capabilities qualified on it, and a new one declares none.
func replaceProtocols(current map[domain.EndpointProtocol][]domain.Capability, protocols []domain.EndpointProtocol) map[domain.EndpointProtocol][]domain.Capability {
	replaced := make(map[domain.EndpointProtocol][]domain.Capability, len(protocols))
	for _, protocol := range protocols {
		replaced[protocol] = slices.Clone(current[protocol])
		if replaced[protocol] == nil {
			replaced[protocol] = []domain.Capability{}
		}
	}
	return replaced
}

// pinSelections records the protocol that each client selecting the model
// resolves to, so that the model's new protocols cannot make an implicit
// selection ambiguous.
func pinSelections(cfg *domain.Config, key string) {
	for _, client := range cfg.SelectedClientsForModel(key) {
		if selected, err := cfg.ResolveRuntime(client, ""); err == nil {
			binding := cfg.Clients[client]
			binding.Protocol = selected.Protocol
			cfg.Clients[client] = binding
		}
	}
}

// declare maps each protocol to the capabilities that it admits, refusing a
// capability that none of the protocols admits.
func declare(ref domain.ModelRef, protocols []domain.EndpointProtocol, capabilities []domain.Capability) (map[domain.EndpointProtocol][]domain.Capability, *unadmittedCapabilityError) {
	declared := make(map[domain.EndpointProtocol][]domain.Capability, len(protocols))
	for _, protocol := range protocols {
		declared[protocol] = []domain.Capability{}
	}
	for _, capability := range capabilities {
		admitted := false
		for _, protocol := range protocols {
			if protocol.Admits(capability) {
				declared[protocol] = append(declared[protocol], capability)
				admitted = true
			}
		}
		if !admitted {
			return nil, &unadmittedCapabilityError{ref: ref, capability: capability, protocols: protocols}
		}
	}
	return declared, nil
}

// unadmittedCapabilityError is a capability that none of a model's protocols
// admits.
type unadmittedCapabilityError struct {
	ref        domain.ModelRef
	capability domain.Capability
	protocols  []domain.EndpointProtocol
}

func (e *unadmittedCapabilityError) Error() string {
	return fmt.Sprintf("no protocol of %s admits %s; it declares %s", e.ref, e.capability, protocolLabels(e.protocols))
}

// capabilityRefusal gives a capability that no protocol of the model admits
// its fix: the same command with the protocol that admits it added, when the
// provider serves that protocol, or else the command's help.
func capabilityRefusal(cmd *cobra.Command, provider domain.Provider, unadmitted *unadmittedCapabilityError) error {
	fix := cmd.CommandPath() + " --help"
	if unadmitted.capability == domain.CapabilityToolSearch && provider.Endpoints.For(domain.ProtocolAnthropic) != "" {
		// add declares one protocol, so its fix replaces it; edit's keeps the
		// model's own protocols beside the one that admits the capability.
		protocols := []domain.EndpointProtocol{domain.ProtocolAnthropic}
		if cmd.Name() == "edit" {
			protocols = append(slices.Clone(unadmitted.protocols), domain.ProtocolAnthropic)
		}
		var words []string
		for _, protocol := range protocols {
			words = append(words, "--protocol", protocol.Name())
		}
		if line := invocation.Rerun(cmd, cmd.Flags().Args(), []string{"protocol"}, words...); line != "" {
			fix = line
		}
	}
	return terminal.ProblemError(unadmitted.Error(), "", "", fix, unadmitted)
}

// parseCapabilities parses repeated --capability values into a set in the
// order in which AIGW lists the capabilities. An empty value clears the set
// and may only stand alone.
func parseCapabilities(cmd *cobra.Command, values []string) ([]domain.Capability, error) {
	if len(values) == 1 && values[0] == "" {
		return []domain.Capability{}, nil
	}
	given := map[domain.Capability]bool{}
	for _, value := range values {
		capability := domain.Capability(value)
		if !capability.Known() {
			return nil, fmt.Errorf("invalid argument %q for \"--capability\" flag: expected %s; run `%s --help`", value, capabilityUsage(), cmd.CommandPath())
		}
		given[capability] = true
	}
	return slices.DeleteFunc(domain.Capabilities(), func(capability domain.Capability) bool { return !given[capability] }), nil
}

// capabilityUsage names the capabilities that AIGW defines.
func capabilityUsage() string {
	names := make([]string, 0, len(domain.Capabilities()))
	for _, capability := range domain.Capabilities() {
		names = append(names, string(capability))
	}
	return strings.Join(names, ", ")
}
