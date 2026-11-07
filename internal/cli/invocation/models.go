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
// providers when its provider is unknown.
func ResolveModel(cfg domain.Config, ref domain.ModelRef) (string, domain.Model, error) {
	if _, err := ResolveProvider(cfg, ref.Provider()); err != nil {
		return "", domain.Model{}, err
	}
	if key, found := cfg.ResolveModelRef(ref); found {
		return key, cfg.Models[key], nil
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
