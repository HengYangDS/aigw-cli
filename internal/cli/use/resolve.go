package use

import (
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// choice is the selection that the arguments, prompts and --protocol
// resolve: the client, the model's name and its key, and the protocol.
// chosen marks a protocol that --protocol or a prompt chose among several,
// which a command line that repeats the selection names. adds marks a model
// that the selection adds, and gains an added model to which it gives the
// protocol. unlisted marks a selection that adds without the catalog check,
// which a command line that repeats it names too.
type choice struct {
	client   domain.ClientSpec
	ref      domain.ModelRef
	protocol domain.EndpointProtocol
	chosen   bool
	adds     bool
	gains    bool
	unlisted bool
}

// command is the aigw use command line that makes this selection with each
// flag that changes its effect, or "" when a word would need quoting. A
// protocol name never does.
func (c choice) command() string {
	line := invocation.Command("use", c.client.ID, c.ref.String())
	if line == "" {
		return ""
	}
	if c.chosen {
		line += " --protocol " + c.protocol.Name()
	}
	if c.unlisted {
		line += " --unlisted"
	}
	return line
}

// resolve resolves the client and the model that the arguments name, asking
// in a terminal for the one that is missing, and then the protocol. A model
// argument without a slash is refused with the models that it could mean, and
// an unknown provider with the closest ones.
func resolve(runtime invocation.Context, cfg domain.Config, given arguments, requested domain.EndpointProtocol) (choice, error) {
	var ref domain.ModelRef
	if given.model != "" {
		parsed, err := invocation.ParseModelArgument(cfg, given.model)
		if err != nil {
			return choice{}, err
		}
		if _, err := invocation.ResolveProvider(cfg, parsed.Provider()); err != nil {
			return choice{}, err
		}
		ref = parsed
	}
	client := given.client
	if client == "" {
		var err error
		if client, err = chooseClient(runtime, cfg, ref, given.model != ""); err != nil {
			return choice{}, err
		}
	}
	spec, _ := runtime.Clients.Admission().Spec(client)
	if given.model == "" {
		var err error
		if ref, err = chooseModel(runtime, cfg, spec); err != nil {
			return choice{}, err
		}
	}
	return chooseProtocol(runtime, cfg, spec, ref, requested)
}

// chooseClient asks for the client: any client, or for a named model one that
// speaks a protocol that the model's provider serves.
func chooseClient(runtime invocation.Context, cfg domain.Config, ref domain.ModelRef, named bool) (string, error) {
	provider := cfg.Providers[ref.Provider()]
	choices := []terminal.Choice{}
	for _, spec := range runtime.Clients.Admission().Specs() {
		if !named || len(spec.CompatibleProtocols(provider)) > 0 {
			choices = append(choices, terminal.Choice{Value: spec.ID, Label: spec.Label})
		}
	}
	if len(choices) == 0 {
		return "", terminal.ProblemError(fmt.Sprintf("no client speaks a protocol that provider %s serves", ref.Provider()), "", "", "aigw provider show "+ref.Provider(), nil)
	}
	client, err := runtime.Prompt.Select("Select the client: ", choices)
	return client, invocation.Prompted("The client prompt", err)
}

// chooseModel asks for one of the added models that the client can use, those
// whose provider serves a protocol that the client speaks: the models that
// the client's recommendation names first, in its order, and then the others
// in configuration order. Each model is offered once, by its name.
func chooseModel(runtime invocation.Context, cfg domain.Config, spec domain.ClientSpec) (domain.ModelRef, error) {
	refs := []domain.ModelRef{}
	for _, selection := range cfg.Recommendations[spec.ID].Selections() {
		refs = append(refs, selection.Model)
	}
	recommended := len(refs)
	refs = append(refs, cfg.ModelRefs()...)
	choices, offered := []terminal.Choice{}, map[domain.ModelRef]bool{}
	for index, ref := range refs {
		model, added := cfg.Models[ref]
		if !added || offered[ref] || len(spec.CompatibleProtocols(cfg.Providers[model.Provider])) == 0 {
			continue
		}
		offered[ref] = true
		name := ref.String()
		label := name + " · " + cfg.ModelLabel(ref)
		if index < recommended {
			label += " · recommended"
		}
		choices = append(choices, terminal.Choice{Value: name, Label: label})
	}
	if len(choices) == 0 {
		return domain.ModelRef{}, terminal.ProblemError("no added model can serve "+spec.Label+": no provider of an added model serves a protocol that it speaks",
			"", "", "aigw model search", nil)
	}
	selected, err := runtime.Prompt.Select("Select the model for "+spec.Label+": ", choices)
	if err != nil {
		return domain.ModelRef{}, invocation.Prompted("The model prompt for "+spec.Label, err)
	}
	return domain.ParseModelRef(selected)
}

// chooseProtocol resolves the protocol on which the client uses the model: --protocol, the only protocol that the model declares, the
// client speaks and the provider serves, or among several the client's
// current protocol, the one that its recommendation of this model names, or
// one that a prompt asks for. Without a prompt, several protocols are refused
// with the choice to make. A model that is not added, or that declares none
// of those protocols, or not the one that --protocol names, gets one of the
// protocols that the client speaks and the provider serves, chosen the same
// way. No protocol is inferred from the model's ID or another client.
func chooseProtocol(runtime invocation.Context, cfg domain.Config, spec domain.ClientSpec, ref domain.ModelRef, requested domain.EndpointProtocol) (choice, error) {
	provider, _ := invocation.ResolveProvider(cfg, ref.Provider())
	served := spec.CompatibleProtocols(provider)
	if err := admitProtocol(spec, ref, provider, served, requested); err != nil {
		return choice{}, err
	}
	model, added := cfg.Models[ref]
	var options []domain.EndpointProtocol
	for _, protocol := range served {
		if _, declared := model.Protocols[protocol]; declared && (requested == "" || protocol == requested) {
			options = append(options, protocol)
		}
	}
	selected := choice{client: spec, ref: ref, chosen: requested != ""}
	if len(options) == 0 {
		selected.adds, selected.gains = !added, added
		for _, protocol := range served {
			if requested == "" || protocol == requested {
				options = append(options, protocol)
			}
		}
	}
	if len(options) == 1 {
		selected.protocol = options[0]
		return selected, nil
	}
	if preferred, found := preferredProtocol(cfg, spec, ref, options); found {
		selected.protocol = preferred
		return selected, nil
	}
	picked, err := askProtocol(runtime, spec, ref, options)
	if err != nil {
		return choice{}, err
	}
	selected.protocol, selected.chosen = picked, true
	return selected, nil
}

// admitProtocol refuses, before credential or network access, a selection
// that no protocol can carry: a provider that serves none of the protocols
// that the client speaks, and a --protocol that the client does not speak or
// that the provider does not serve.
func admitProtocol(spec domain.ClientSpec, ref domain.ModelRef, provider domain.Provider, served []domain.EndpointProtocol, requested domain.EndpointProtocol) error {
	spoken := protocolLabels(spec.EndpointProtocols)
	switch {
	case len(served) == 0:
		return terminal.ProblemError(fmt.Sprintf("%s cannot use %s: it speaks %s, and provider %s has no endpoint for any of them",
			spec.Label, ref, spoken, provider.ID), "", "", "aigw provider edit "+provider.ID+" --endpoint "+protocolChoice(spec.EndpointProtocols)+"=<url>", nil)
	case requested == "":
		return nil
	case !slices.Contains(spec.EndpointProtocols, requested):
		return terminal.ProblemError(fmt.Sprintf("%s does not speak %s; it speaks %s", spec.Label, domain.ProtocolLabel(requested), spoken),
			"", "", "aigw use --help", nil)
	case !slices.Contains(served, requested):
		return terminal.ProblemError(fmt.Sprintf("provider %s has no %s endpoint for %s", provider.ID, domain.ProtocolLabel(requested), ref),
			"", "", "aigw provider edit "+provider.ID+" --endpoint "+requested.Name()+"=<url>", nil)
	}
	return nil
}

// preferredProtocol is the client's current protocol, or else the protocol
// that the client's recommendation of this model names, when it is one of
// options.
func preferredProtocol(cfg domain.Config, spec domain.ClientSpec, ref domain.ModelRef, options []domain.EndpointProtocol) (domain.EndpointProtocol, bool) {
	var preferred []domain.EndpointProtocol
	if current, err := cfg.ResolveRuntime(spec.ID, domain.ModelRef{}); err == nil {
		preferred = append(preferred, current.Protocol)
	}
	for _, selection := range cfg.Recommendations[spec.ID].Selections() {
		if _, added := cfg.Models[selection.Model]; added && selection.Model == ref && selection.Protocol != "" {
			preferred = append(preferred, selection.Protocol)
		}
	}
	for _, protocol := range preferred {
		if slices.Contains(options, protocol) {
			return protocol, true
		}
	}
	return "", false
}

// askProtocol asks for one of several protocols in a terminal, and without
// one refuses with the command that names the choice.
func askProtocol(runtime invocation.Context, spec domain.ClientSpec, ref domain.ModelRef, protocols []domain.EndpointProtocol) (domain.EndpointProtocol, error) {
	names, choices := []string{}, []terminal.Choice{}
	for _, protocol := range protocols {
		names = append(names, protocol.Name())
		choices = append(choices, terminal.Choice{Value: protocol.Name(), Label: domain.ProtocolLabel(protocol)})
	}
	if !runtime.Interactive {
		return "", terminal.ProblemError(fmt.Sprintf("%s can use %s on %s, so the protocol must be chosen", spec.Label, ref, protocolLabels(protocols)),
			"", "", fmt.Sprintf("aigw use %s %s --protocol %s", spec.ID, ref, protocolChoice(protocols)), nil)
	}
	picked, err := runtime.Prompt.Select("Select the protocol for "+ref.String()+": ", choices)
	if err != nil {
		return "", invocation.Prompted("The protocol prompt for "+ref.String(), err)
	}
	protocol, known := domain.ProtocolNamed(picked)
	if !known || !slices.Contains(protocols, protocol) {
		return "", fmt.Errorf("protocol %q is not one of %s", picked, strings.Join(names, ", "))
	}
	return protocol, nil
}

// protocolLabels names protocols by their labels, in a list for prose.
func protocolLabels(protocols []domain.EndpointProtocol) string {
	labels := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		labels = append(labels, domain.ProtocolLabel(protocol))
	}
	switch len(labels) {
	case 0:
		return "no protocol"
	case 1:
		return labels[0]
	default:
		return strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
	}
}

// protocolChoice is a protocol to type: the one name, or a placeholder of the
// choice among several, which makes a fix a next_detail.
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
