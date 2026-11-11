package domain

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// displayOrder is the order of a configuration's providers, families and
// models that its file gives: the file's arrays are its display order, so
// every list follows the file. The store records it as it reads the file
// through the Append methods. An entry that a command adds to a map directly
// comes after the recorded ones, in identity order, so that no list depends
// on map order.
type displayOrder struct {
	providers []string
	families  []string
	models    []ModelRef
}

func (order displayOrder) clone() displayOrder {
	return displayOrder{providers: slices.Clone(order.providers), families: slices.Clone(order.families), models: slices.Clone(order.models)}
}

// ProviderIDs returns the providers in display order.
func (c *Config) ProviderIDs() []string {
	return inDisplayOrder(c.order.providers, c.Providers, strings.Compare)
}

// FamilyIDs returns the families in display order.
func (c *Config) FamilyIDs() []string {
	return inDisplayOrder(c.order.families, c.Families, strings.Compare)
}

// ModelRefs returns every model in display order: grouped by provider in the
// providers' order, and within one provider in the order of its models.
func (c *Config) ModelRefs() []ModelRef {
	refs := inDisplayOrder(c.order.models, c.Models, compareModelRefs)
	rank := make(map[string]int, len(c.Providers))
	for index, id := range c.ProviderIDs() {
		rank[id] = index
	}
	slices.SortStableFunc(refs, func(left, right ModelRef) int {
		leftRank, leftKnown := rank[left.provider]
		rightRank, rightKnown := rank[right.provider]
		switch {
		case leftKnown && rightKnown:
			return cmp.Compare(leftRank, rightRank)
		case leftKnown != rightKnown:
			// A model of an unknown provider, which validation refuses,
			// comes last.
			if leftKnown {
				return -1
			}
			return 1
		default:
			return strings.Compare(left.provider, right.provider)
		}
	})
	return refs
}

// ProviderModelRefs returns the models of one provider in display order.
func (c *Config) ProviderModelRefs(id string) []ModelRef {
	return slices.DeleteFunc(c.ModelRefs(), func(ref ModelRef) bool { return ref.provider != id })
}

// AppendProvider adds a provider that the configuration does not have yet,
// last in display order. An ID that it has already is refused, as a file
// that names one provider twice.
func (c *Config) AppendProvider(id string, provider Provider) error {
	if _, exists := c.Providers[id]; exists {
		return fmt.Errorf("provider %q is defined twice", id)
	}
	c.order.providers = append(c.ProviderIDs(), id)
	if c.Providers == nil {
		c.Providers = map[string]Provider{}
	}
	c.Providers[id] = provider
	return nil
}

// AppendFamily adds a family that the configuration does not have yet, last
// in display order. An ID that it has already is refused.
func (c *Config) AppendFamily(id string, family Family) error {
	if _, exists := c.Families[id]; exists {
		return fmt.Errorf("family %q is defined twice", id)
	}
	c.order.families = append(c.FamilyIDs(), id)
	if c.Families == nil {
		c.Families = map[string]Family{}
	}
	c.Families[id] = family
	return nil
}

// AppendModel adds a model that the configuration does not have yet, last
// among its provider's models in display order, under its reference. A
// provider's model ID that it has already is refused.
func (c *Config) AppendModel(model Model) error {
	ref := model.Ref()
	if _, exists := c.Models[ref]; exists {
		return fmt.Errorf("provider %q defines model %q twice", model.Provider, model.ID)
	}
	c.order.models = append(c.ModelRefs(), ref)
	if c.Models == nil {
		c.Models = map[ModelRef]Model{}
	}
	c.Models[ref] = model
	return nil
}

// normalizeOrder records the display order of every entry, so that two
// configurations with the same entries in the same order are equal.
func (c *Config) normalizeOrder() {
	c.order = displayOrder{
		providers: slices.Clip(nilWhenEmpty(c.ProviderIDs())),
		families:  slices.Clip(nilWhenEmpty(c.FamilyIDs())),
		models:    slices.Clip(nilWhenEmpty(c.ModelRefs())),
	}
}

func nilWhenEmpty[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return values
}

// inDisplayOrder returns the keys of entries: those that recorded names, in
// its order, then the others sorted by compare.
func inDisplayOrder[K comparable, V any](recorded []K, entries map[K]V, compare func(K, K) int) []K {
	ordered := make([]K, 0, len(entries))
	listed := make(map[K]bool, len(entries))
	for _, key := range recorded {
		if _, exists := entries[key]; exists && !listed[key] {
			ordered = append(ordered, key)
			listed[key] = true
		}
	}
	rest := slices.SortedFunc(maps.Keys(entries), compare)
	return append(ordered, slices.DeleteFunc(rest, func(key K) bool { return listed[key] })...)
}

func compareModelRefs(left, right ModelRef) int {
	return cmp.Or(strings.Compare(left.provider, right.provider), strings.Compare(left.id, right.id))
}

// CompareProtocols orders protocols as output names them: anthropic,
// responses, then chat.
func CompareProtocols(left, right EndpointProtocol) int {
	return cmp.Compare(slices.Index(knownProtocols, left), slices.Index(knownProtocols, right))
}
