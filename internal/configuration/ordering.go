package configuration

import (
	"bytes"
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"aigw-cli/internal/transaction"

	"github.com/pelletier/go-toml/v2"
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

// catalogOrder is the Store's presentation component. Published credential
// readers consume the unchanged canonical schema, never this component.
type catalogOrder struct {
	Accounts map[string]int `toml:"accounts,omitempty"`
	Models   map[string]int `toml:"models,omitempty"`
}

func (s Store) catalogPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".catalog.toml"
}

func encodeCatalogOrder(cfg Config) ([]byte, error) {
	order := catalogOrder{Accounts: map[string]int{}, Models: map[string]int{}}
	for id, account := range cfg.Accounts {
		if account.Order != 0 {
			order.Accounts[id] = account.Order
		}
	}
	for id, model := range cfg.Models {
		if model.Order != 0 {
			order.Models[id] = model.Order
		}
	}
	if len(order.Accounts)+len(order.Models) == 0 {
		return nil, nil
	}
	data, err := toml.Marshal(order)
	return separateTOMLTableBlocks(data), err
}

func decodeCatalogOrder(cfg Config, snapshot transaction.FileSnapshot) (Config, error) {
	if !snapshot.Exists {
		return cfg, nil
	}
	var order catalogOrder
	decoder := toml.NewDecoder(bytes.NewReader(snapshot.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&order); err != nil {
		return Config{}, fmt.Errorf("parse catalogue presentation: %w", err)
	}
	for id, rank := range order.Accounts {
		if account, exists := cfg.Accounts[id]; exists {
			account.Order = rank
			cfg.Accounts[id] = account
		}
	}
	for id, rank := range order.Models {
		if model, exists := cfg.Models[id]; exists {
			model.Order = rank
			cfg.Models[id] = model
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (s Store) loadSnapshot(snapshot Snapshot) (Config, error) {
	cfg, err := decodeStoreConfig(snapshot.Config.Data, snapshot.Forwarding, s.admission)
	if err != nil {
		return Config{}, err
	}
	cfg, err = decodeComponents(cfg, snapshot.Catalog, snapshot.Capabilities)
	if err != nil {
		return Config{}, err
	}
	return decodeRetryPolicies(cfg, snapshot.Retries)
}
