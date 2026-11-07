package invocation

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/domain"
)

// refusal is an unknown name or a malformed argument that a command refuses
// with its own fix: the command that lists or shows what the user can name.
type refusal struct {
	message string
	fix     string
}

func (e refusal) Error() string { return e.message }

// NextAction is the refusal's own fix.
func (e refusal) NextAction() string { return e.fix }

// ParseModelArgument parses a `provider/model` argument. An argument without a
// slash is refused with each added name whose model ID it is, once, when there
// are any, and the list of models that it could mean.
func ParseModelArgument(cfg domain.Config, text string) (domain.ModelRef, error) {
	ref, err := domain.ParseModelRef(text)
	if !errors.Is(err, domain.ErrModelRefWithoutProvider) {
		return ref, err
	}
	var named []string
	for _, key := range cfg.CatalogModelIDs() {
		name := cfg.Models[key].Ref().String()
		if strings.EqualFold(cfg.Models[key].UpstreamModelID(), text) && !slices.Contains(named, name) {
			named = append(named, name)
		}
	}
	fix := "aigw model list " + text
	if !plain(text) {
		fix = "aigw model list"
	}
	if len(named) == 0 {
		return domain.ModelRef{}, refusal{message: fmt.Sprintf("%q names no provider; a model is named provider/model", text), fix: fix}
	}
	return domain.ModelRef{}, refusal{message: fmt.Sprintf("%q names no provider; it could mean %s", text, strings.Join(named, ", ")), fix: fix}
}

// ResolveModel returns the configured model that ref names and its key, which
// only the configuration and its transactions use. An unknown model is
// refused with the provider's closest model IDs, or with the closest
// providers when its provider is unknown. A given protocol always selects:
// a model that does not declare it is refused. Today's configuration can
// hold one model of a name per protocol, and a name that still names several
// is refused with the command, rerun, that selects one.
func ResolveModel(cfg domain.Config, ref domain.ModelRef, protocol domain.EndpointProtocol, rerun func(choice string) string) (string, domain.Model, error) {
	if _, err := ResolveProvider(cfg, ref.Provider()); err != nil {
		return "", domain.Model{}, err
	}
	named := cfg.ResolveModelRef(ref)
	if len(named) == 0 {
		return "", domain.Model{}, unknownModel(cfg, ref)
	}
	keys := slices.Clone(named)
	if protocol != "" {
		keys = slices.DeleteFunc(keys, func(key string) bool {
			_, declared := cfg.Models[key].Protocols[protocol]
			return !declared
		})
	}
	switch len(keys) {
	case 0:
		fix := "aigw model list --provider " + ref.Provider()
		if show := Command("model", "show", ref.String()); len(named) == 1 && show != "" {
			fix = show
		}
		return "", domain.Model{}, refusal{message: fmt.Sprintf("%s declares no %s", ref, domain.ProtocolLabel(protocol)), fix: fix}
	case 1:
		return keys[0], cfg.Models[keys[0]], nil
	default:
		return "", domain.Model{}, ambiguousModel(cfg, ref, keys, rerun)
	}
}

// unknownModel refuses a name that no configured model carries, with the
// provider's closest model IDs.
func unknownModel(cfg domain.Config, ref domain.ModelRef) error {
	var ids []string
	for _, key := range cfg.CatalogModelIDs() {
		if model := cfg.Models[key]; model.Provider == ref.Provider() {
			ids = append(ids, model.UpstreamModelID())
		}
	}
	return refusal{
		message: "unknown model " + ref.String() + closestText(ref.ID(), ids, ref.Provider()+"/"),
		fix:     "aigw model list --provider " + ref.Provider(),
	}
}

// ambiguousModel refuses a name that several configured models carry. When
// a protocol selects each of them, it names those protocols, with the command
// rerun with --protocol, or `aigw model show` for a command whose --protocol
// means something else. Otherwise it names each model's configuration ID, which
// tells them apart until task 6.1 merges them, and the configuration's path.
// The choice is always the user's: a fix never selects one of the models.
func ambiguousModel(cfg domain.Config, ref domain.ModelRef, keys []string, rerun func(choice string) string) error {
	protocols, selectsEach := selectingProtocols(cfg, keys)
	if !selectsEach {
		return refusal{
			message: fmt.Sprintf("%s names %d models, %s, and no protocol selects each of them", ref, len(keys), strings.Join(keys, ", ")),
			fix:     "aigw config path",
		}
	}
	names := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		names = append(names, protocol.Name())
	}
	choice := "<" + strings.Join(names, "|") + ">"
	fix := ""
	if rerun != nil {
		fix = rerun(choice)
	} else if show := Command("model", "show", ref.String()); show != "" {
		fix = show + " --protocol " + choice
	}
	if fix == "" {
		fix = "Run the command for one of them with --protocol " + choice + "."
	}
	return refusal{message: fmt.Sprintf("%s names %d models; each of the protocols %s selects one", ref, len(keys), strings.Join(names, ", ")), fix: fix}
}

// selectingProtocols returns, in display order, the protocols that exactly
// one of keys declares, and whether each of keys declares one of them, so that
// a protocol selects each model.
func selectingProtocols(cfg domain.Config, keys []string) ([]domain.EndpointProtocol, bool) {
	declarers := map[domain.EndpointProtocol]int{}
	for _, key := range keys {
		for protocol := range cfg.Models[key].Protocols {
			declarers[protocol]++
		}
	}
	var unique []domain.EndpointProtocol
	for protocol, count := range declarers {
		if count == 1 {
			unique = append(unique, protocol)
		}
	}
	slices.SortFunc(unique, domain.CompareProtocols)
	for _, key := range keys {
		if !slices.ContainsFunc(unique, func(protocol domain.EndpointProtocol) bool {
			_, declared := cfg.Models[key].Protocols[protocol]
			return declared
		}) {
			return unique, false
		}
	}
	return unique, true
}

// ResolveProvider returns the configured provider with id. An unknown one is
// refused with the closest provider IDs; its fix lists the providers.
func ResolveProvider(cfg domain.Config, id string) (domain.Provider, error) {
	if provider, found := cfg.Providers[id]; found {
		provider.ID = id
		return provider, nil
	}
	return domain.Provider{}, refusal{
		message: fmt.Sprintf("unknown provider %q", id) + closestText(id, cfg.CatalogProviderIDs(), ""),
		fix:     "aigw provider list",
	}
}

// ResolveClient returns the admitted client with id. An unknown one is refused
// with the closest client IDs and every client that the admission names; its
// fix lists the clients.
func ResolveClient(admission domain.Admission, id string) (domain.ClientSpec, error) {
	if spec, admitted := admission.Spec(id); admitted {
		return spec, nil
	}
	message := fmt.Sprintf("unknown client %q; clients are %s", id, admission.Usage())
	if closest := ClosestMatches(id, admission.IDs()); len(closest) > 0 {
		message = fmt.Sprintf("unknown client %q; did you mean %s? Clients are %s", id, strings.Join(closest, ", "), admission.Usage())
	}
	return domain.ClientSpec{}, refusal{message: message, fix: "aigw client list"}
}

// closestText is the refusal text that names the closest of candidates to
// typed, each with prefix, or nothing when none is close.
func closestText(typed string, candidates []string, prefix string) string {
	closest := ClosestMatches(typed, slices.Compact(slices.Sorted(slices.Values(candidates))))
	if len(closest) == 0 {
		return ""
	}
	for index, candidate := range closest {
		closest[index] = prefix + candidate
	}
	return "; did you mean " + strings.Join(closest, ", ") + "?"
}
