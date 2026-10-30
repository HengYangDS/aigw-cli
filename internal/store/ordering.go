package store

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"github.com/pelletier/go-toml/v2"
)

// catalogOrder is the Store's presentation component. Published credential
// readers consume the unchanged canonical schema, never this component.
type catalogOrder struct {
	Accounts map[string]int `toml:"accounts,omitempty"`
	Models   map[string]int `toml:"models,omitempty"`
}

func (s Store) catalogPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".catalog.toml"
}

func encodeCatalogOrder(cfg domain.Config) ([]byte, error) {
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

func decodeCatalogOrder(cfg domain.Config, snapshot platform.FileSnapshot) (domain.Config, error) {
	if !snapshot.Exists {
		return cfg, nil
	}
	var order catalogOrder
	decoder := toml.NewDecoder(bytes.NewReader(snapshot.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&order); err != nil {
		return domain.Config{}, fmt.Errorf("parse catalogue presentation: %w", err)
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
		return domain.Config{}, err
	}
	return cfg, nil
}

func (s Store) loadSnapshot(snapshot Snapshot) (domain.Config, error) {
	cfg, err := decodeStoreConfig(snapshot.Config.Data, snapshot.Forwarding, s.admission)
	if err != nil {
		return domain.Config{}, err
	}
	cfg, err = decodeComponents(cfg, snapshot.Catalog, snapshot.capabilities)
	if err != nil {
		return domain.Config{}, err
	}
	return decodeRetryPolicies(cfg, snapshot.retries)
}
