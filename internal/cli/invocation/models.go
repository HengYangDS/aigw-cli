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
	for _, key := range cfg.ModelRefs() {
		if strings.EqualFold(key.ID(), text) {
			named = append(named, key.String())
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

// ResolveModel returns the configured model that ref names. An unknown model
// is refused with the provider's closest model IDs, or with the closest
// providers when its provider is unknown. A given protocol always selects: a
// model that does not declare it is refused.
func ResolveModel(cfg domain.Config, ref domain.ModelRef, protocol domain.EndpointProtocol) (domain.Model, error) {
	if _, err := ResolveProvider(cfg, ref.Provider()); err != nil {
		return domain.Model{}, err
	}
	model, added := cfg.Models[ref]
	if !added {
		return domain.Model{}, unknownModel(cfg, ref)
	}
	if _, declared := model.Protocols[protocol]; protocol != "" && !declared {
		fix := "aigw model list --provider " + ref.Provider()
		if show := Command("model", "show", ref.String()); show != "" {
			fix = show
		}
		return domain.Model{}, refusal{message: fmt.Sprintf("%s declares no %s", ref, domain.ProtocolLabel(protocol)), fix: fix}
	}
	return model, nil
}

// unknownModel refuses a name that no configured model carries, with the
// provider's closest model IDs.
func unknownModel(cfg domain.Config, ref domain.ModelRef) error {
	var ids []string
	for _, key := range cfg.ProviderModelRefs(ref.Provider()) {
		ids = append(ids, key.ID())
	}
	return refusal{
		message: "unknown model " + ref.String() + closestText(ref.ID(), ids, ref.Provider()+"/"),
		fix:     "aigw model list --provider " + ref.Provider(),
	}
}

// ResolveProvider returns the configured provider with id. An unknown one is
// refused with the closest provider IDs; its fix lists the providers.
func ResolveProvider(cfg domain.Config, id string) (domain.Provider, error) {
	if provider, found := cfg.Providers[id]; found {
		provider.ID = id
		return provider, nil
	}
	return domain.Provider{}, refusal{
		message: fmt.Sprintf("unknown provider %q", id) + closestText(id, cfg.ProviderIDs(), ""),
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
	if closest := closestMatches(id, admission.IDs()); len(closest) > 0 {
		message = fmt.Sprintf("unknown client %q; did you mean %s? Clients are %s", id, strings.Join(closest, ", "), admission.Usage())
	}
	return domain.ClientSpec{}, refusal{message: message, fix: "aigw client list"}
}

// ResolveLoneClient returns the admitted client with id, a lone argument
// without a slash that a command taking a client or a model was given. An
// unknown one is refused as ResolveClient refuses it, from the admission
// alone and before any configuration access. A lone word that names no
// client is most often a model ID without its provider, so the refusal's fix
// is the model list that it may have meant, which runs as given and reads the
// configuration, and the clients' list stays in its text.
func ResolveLoneClient(admission domain.Admission, id string) (domain.ClientSpec, error) {
	spec, err := ResolveClient(admission, id)
	failed, refused := errors.AsType[refusal](err)
	if !refused {
		return spec, err
	}
	failed.fix = Command("model", "list", id)
	if failed.fix == "" {
		failed.fix = "aigw model list"
	}
	failed.message += ", as `aigw client list` shows; a model is named provider/model"
	return domain.ClientSpec{}, failed
}

// closestText is the refusal text that names the closest of candidates to
// typed, each with prefix, or nothing when none is close.
func closestText(typed string, candidates []string, prefix string) string {
	closest := closestMatches(typed, slices.Compact(slices.Sorted(slices.Values(candidates))))
	if len(closest) == 0 {
		return ""
	}
	for index, candidate := range closest {
		closest[index] = prefix + candidate
	}
	return "; did you mean " + strings.Join(closest, ", ") + "?"
}
