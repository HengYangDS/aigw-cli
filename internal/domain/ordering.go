package domain

import (
	"cmp"
	"slices"
	"strings"
)

// CatalogModelIDs groups display rows by Account, protocol set, and Model order.
// ModelIDs retains canonical lexical enumeration for identity and diagnostics.
func (c *Config) CatalogModelIDs() []string {
	ids := c.ModelIDs()
	protocols := make(map[string][]EndpointProtocol, len(ids))
	for _, id := range ids {
		protocols[id] = c.Models[id].AdmittedProtocols()
		slices.SortFunc(protocols[id], CompareProtocols)
	}
	slices.SortFunc(ids, func(left, right string) int {
		l, r := c.Models[left], c.Models[right]
		if result := c.CompareProviders(l.Provider, r.Provider); result != 0 {
			return result
		}
		if result := slices.CompareFunc(protocols[left], protocols[right], CompareProtocols); result != 0 {
			return result
		}
		if result := c.CompareFamilies(l.Family, r.Family); result != 0 {
			return result
		}
		return strings.Compare(left, right)
	})
	return ids
}

// CompareProviders orders declared Accounts, then undeclared ranks by identity.
func (c *Config) CompareProviders(left, right string) int {
	if result := compareOrder(c.Providers[left].Order, c.Providers[right].Order); result != 0 {
		return result
	}
	return strings.Compare(left, right)
}

// CatalogProviderIDs returns deterministic presentation order for Account rows.
func (c *Config) CatalogProviderIDs() []string {
	ids := make([]string, 0, len(c.Providers))
	for id := range c.Providers {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, c.CompareProviders)
	return ids
}

// CompareFamilies orders canonical Models without interpreting provider wire IDs.
func (c *Config) CompareFamilies(left, right string) int {
	if result := compareOrder(c.Families[left].Order, c.Families[right].Order); result != 0 {
		return result
	}
	return strings.Compare(left, right)
}

func compareOrder(left, right int) int {
	if left == 0 && right != 0 {
		return 1
	}
	if right == 0 && left != 0 {
		return -1
	}
	return cmp.Compare(left, right)
}

// CompareProtocols groups native display protocols independently of routing preference.
func CompareProtocols(left, right EndpointProtocol) int {
	order := []EndpointProtocol{ProtocolAnthropic, ProtocolOpenAIResponses, ProtocolOpenAIChatCompletions}
	return cmp.Compare(slices.Index(order, left), slices.Index(order, right))
}
