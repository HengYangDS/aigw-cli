package store

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

type retryPolicies struct {
	Clients map[string]retryConnection `toml:"clients"`
}

type retryConnection struct {
	Identity string             `toml:"connection_identity"`
	Policy   domain.RetryPolicy `toml:"retry_policy"`
}

func (s Store) retriesPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".retries.toml"
}

func encodeRetryPolicies(cfg domain.Config) ([]byte, error) {
	policies := retryPolicies{Clients: map[string]retryConnection{}}
	for client, binding := range cfg.Clients {
		if binding.RetryPolicy == nil {
			continue
		}
		runtime, err := cfg.ResolveRuntime(client, "")
		if err != nil {
			return nil, err
		}
		policies.Clients[client] = retryConnection{Identity: runtime.ConnectionFingerprint(), Policy: *binding.RetryPolicy.Clone()}
	}
	if len(policies.Clients) == 0 {
		return nil, nil
	}
	data, err := toml.Marshal(policies)
	if err != nil {
		return nil, fmt.Errorf("encode client retry policies: %w", err)
	}
	return separateTOMLTableBlocks(data), nil
}

func decodeRetryPolicies(cfg domain.Config, component FileSnapshot) (domain.Config, error) {
	if !component.Exists {
		return cfg, nil
	}
	var policies retryPolicies
	if err := toml.NewDecoder(bytes.NewReader(component.Data)).DisallowUnknownFields().Decode(&policies); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseParse, fmt.Errorf("parse client retry policies: %w", err))
	}
	if len(policies.Clients) == 0 {
		return domain.Config{}, newLoadError(LoadPhaseValidate, errors.New("client retry policy component has no policies"))
	}
	for client, connection := range policies.Clients {
		binding, exists := cfg.Clients[client]
		runtime, err := cfg.ResolveRuntime(client, "")
		if !exists || err != nil || runtime.ConnectionFingerprint() != connection.Identity {
			return domain.Config{}, newLoadError(LoadPhaseValidate, fmt.Errorf("client %q retry policy does not match its selected connection", client))
		}
		if binding.RetryPolicy != nil && !reflect.DeepEqual(binding.RetryPolicy, &connection.Policy) {
			return domain.Config{}, newLoadError(LoadPhaseValidate, fmt.Errorf("client %q retry policy conflicts with its recorded declaration", client))
		}
		binding.RetryPolicy = connection.Policy.Clone()
		cfg.Clients[client] = binding
	}
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseValidate, err)
	}
	return cfg, nil
}

// RollbackConfiguration supplies the retained program's native reader with the
// complete retry declaration in an isolated admission document. Persistent
// canonical bytes stay predecessor-readable; a reader that ignores optional
// components cannot silently pass this capability probe.
func (s Store) RollbackConfiguration() ([]byte, error) {
	snapshot, err := s.CaptureSnapshot()
	if err != nil || !snapshot.Config.Exists {
		return nil, err
	}
	if !snapshot.retries.Exists {
		return snapshot.Config.Data, nil
	}
	cfg, err := s.loadSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := toml.Unmarshal(snapshot.Config.Data, &document); err != nil {
		return nil, err
	}
	clients, ok := document["clients"].(map[string]any)
	if !ok {
		return nil, errors.New("rollback configuration has no client bindings")
	}
	for client, binding := range cfg.Clients {
		if binding.RetryPolicy != nil {
			declaration, ok := clients[client].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("rollback configuration has no %q client binding", client)
			}
			declaration["retry_policy"] = *binding.RetryPolicy.Clone()
		}
	}
	return toml.Marshal(document)
}
