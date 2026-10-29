package domain

import (
	"cmp"
	"slices"
	"strings"
)

// CatalogRouteIDs groups display rows by Account, protocol set, and Model order.
// RouteIDs retains canonical lexical enumeration for identity and diagnostics.
func (c *Config) CatalogRouteIDs() []string {
	ids := c.RouteIDs()
	protocols := make(map[string][]EndpointProtocol, len(ids))
	for _, id := range ids {
		protocols[id] = c.Routes[id].AdmittedProtocols()
		slices.SortFunc(protocols[id], CompareProtocols)
	}
	slices.SortFunc(ids, func(left, right string) int {
		l, r := c.Routes[left], c.Routes[right]
		if result := c.CompareAccounts(l.Account, r.Account); result != 0 {
			return result
		}
		if result := slices.CompareFunc(protocols[left], protocols[right], CompareProtocols); result != 0 {
			return result
		}
		if result := c.CompareModels(l.Model, r.Model); result != 0 {
			return result
		}
		return strings.Compare(left, right)
	})
	return ids
}

// CompareAccounts orders declared Accounts, then undeclared ranks by identity.
func (c *Config) CompareAccounts(left, right string) int {
	if result := compareOrder(c.Accounts[left].Order, c.Accounts[right].Order); result != 0 {
		return result
	}
	return strings.Compare(left, right)
}

// CatalogAccountIDs returns deterministic presentation order for Account rows.
func (c *Config) CatalogAccountIDs() []string {
	ids := make([]string, 0, len(c.Accounts))
	for id := range c.Accounts {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, c.CompareAccounts)
	return ids
}

// CompareModels orders canonical Models without interpreting provider wire IDs.
func (c *Config) CompareModels(left, right string) int {
	if result := compareOrder(c.Models[left].Order, c.Models[right].Order); result != 0 {
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
