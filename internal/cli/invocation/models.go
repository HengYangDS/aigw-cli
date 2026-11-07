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
// slash is refused with the added models whose ID it names, when there are
// any, and the list of models that it could mean.
func ParseModelArgument(cfg domain.Config, text string) (domain.ModelRef, error) {
	ref, err := domain.ParseModelRef(text)
	if !errors.Is(err, domain.ErrModelRefWithoutProvider) {
		return ref, err
	}
	var named []string
	for _, key := range cfg.CatalogModelIDs() {
		if model := cfg.Models[key]; strings.EqualFold(model.UpstreamModelID(), text) {
			named = append(named, model.Ref().String())
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
// providers when its provider is unknown. Today's configuration can hold one
// model of a name per protocol: protocol, when given, selects the one that
// declares it, and a name that still names several is refused, naming each
// protocol, with the command that selects one, rerun.
func ResolveModel(cfg domain.Config, ref domain.ModelRef, protocol domain.EndpointProtocol, rerun func(protocol string) string) (string, domain.Model, error) {
	if _, err := ResolveProvider(cfg, ref.Provider()); err != nil {
		return "", domain.Model{}, err
	}
	keys := cfg.ResolveModelRef(ref)
	if protocol != "" && len(keys) > 1 {
		keys = slices.DeleteFunc(keys, func(key string) bool {
			_, declared := cfg.Models[key].Protocols[protocol]
			return !declared
		})
	}
	switch len(keys) {
	case 1:
		return keys[0], cfg.Models[keys[0]], nil
	case 0:
		if len(cfg.ResolveModelRef(ref)) > 0 {
			return "", domain.Model{}, refusal{
				message: fmt.Sprintf("%s declares no %s", ref, domain.ProtocolLabel(protocol)),
				fix:     "aigw model show " + ref.String(),
			}
		}
	default:
		return "", domain.Model{}, ambiguousModel(cfg, ref, keys, rerun)
	}
	var ids []string
	for _, key := range cfg.CatalogModelIDs() {
		if model := cfg.Models[key]; model.Provider == ref.Provider() {
			ids = append(ids, model.UpstreamModelID())
		}
	}
	return "", domain.Model{}, refusal{
		message: "unknown model " + ref.String() + closestText(ref.ID(), ids, ref.Provider()+"/"),
		fix:     "aigw model list --provider " + ref.Provider(),
	}
}

// ambiguousModel refuses a name that several configured models carry, one
// per protocol, naming each protocol. Its fix is the command rerun with the
// protocol that selects one, or the Route that each is until task 6.1 merges
// them.
func ambiguousModel(cfg domain.Config, ref domain.ModelRef, keys []string, rerun func(protocol string) string) error {
	var protocols []string
	for _, key := range keys {
		for _, protocol := range cfg.Models[key].AdmittedProtocols() {
			protocols = append(protocols, protocol.Name())
		}
	}
	slices.Sort(protocols)
	protocols = slices.Compact(protocols)
	message := fmt.Sprintf("%s names %d models, one for each of the protocols %s", ref, len(keys), strings.Join(protocols, ", "))
	fix := ""
	if rerun != nil {
		fix = rerun("<" + strings.Join(protocols, "|") + ">")
	}
	if fix == "" {
		fix = "aigw route show " + keys[0]
	}
	return refusal{message: message, fix: fix}
}

// ResolveProvider returns the configured provider with id. An unknown one is
// refused with the closest provider IDs; its fix lists the added models, whose
// names begin with their providers.
func ResolveProvider(cfg domain.Config, id string) (domain.Provider, error) {
	if provider, found := cfg.Providers[id]; found {
		provider.ID = id
		return provider, nil
	}
	return domain.Provider{}, refusal{
		message: fmt.Sprintf("unknown provider %q", id) + closestText(id, cfg.CatalogProviderIDs(), ""),
		fix:     "aigw model list",
	}
}

// ResolveClient returns the admitted client with id. An unknown one is refused
// with the closest client IDs and every client that the admission names.
func ResolveClient(admission domain.Admission, id string) (domain.ClientSpec, error) {
	if spec, admitted := admission.Spec(id); admitted {
		return spec, nil
	}
	message := fmt.Sprintf("unknown client %q; clients are %s", id, admission.Usage())
	if closest := ClosestMatches(id, admission.IDs()); len(closest) > 0 {
		message = fmt.Sprintf("unknown client %q; did you mean %s? Clients are %s", id, strings.Join(closest, ", "), admission.Usage())
	}
	return domain.ClientSpec{}, refusal{message: message, fix: "aigw status"}
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
