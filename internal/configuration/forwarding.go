package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"aigw-cli/internal/transaction"

	"github.com/pelletier/go-toml/v2"
)

// clientForwarding persists only client destinations that immutable schema-six
// readers cannot accept. The Store assembles these fields into ClientBinding;
// Accounts, Routes and credentials remain in their original owners.
type clientForwarding struct {
	Clients map[string]forwardingDestination `toml:"clients"`
}

type forwardingDestination struct {
	Endpoint         string           `toml:"endpoint"`
	UpstreamIdentity string           `toml:"upstream_identity"`
	Protocol         EndpointProtocol `toml:"protocol"`
}

func (s Store) forwardingPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".forwarding.toml"
}

func encodeForwarding(cfg Config) ([]byte, error) {
	forwarding := clientForwarding{Clients: map[string]forwardingDestination{}}
	for client, binding := range cfg.Clients {
		if binding.ForwardingEndpoint != "" {
			runtime, err := cfg.ResolveRuntime(client, "")
			if err != nil {
				return nil, err
			}
			forwarding.Clients[client] = forwardingDestination{
				Endpoint: binding.ForwardingEndpoint, UpstreamIdentity: runtime.CredentialProjectionFingerprint(client), Protocol: runtime.Protocol,
			}
		}
	}
	if len(forwarding.Clients) == 0 {
		return nil, nil
	}
	data, err := toml.Marshal(forwarding)
	if err != nil {
		return nil, fmt.Errorf("encode client forwarding: %w", err)
	}
	return separateTOMLTableBlocks(data), nil
}

func decodeStoreConfig(data []byte, forwarding transaction.FileSnapshot) (Config, error) {
	cfg, err := decodeTOMLConfig(data)
	if err != nil || !forwarding.Exists {
		return cfg, err
	}
	var destinations clientForwarding
	decoder := toml.NewDecoder(bytes.NewReader(forwarding.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&destinations); err != nil {
		return Config{}, newLoadError(LoadPhaseParse, fmt.Errorf("parse client forwarding: %w", err))
	}
	if len(destinations.Clients) == 0 {
		return Config{}, newLoadError(LoadPhaseValidate, errors.New("client forwarding component has no destinations"))
	}
	for _, client := range slices.Sorted(maps.Keys(destinations.Clients)) {
		binding, exists := cfg.Clients[client]
		destination := destinations.Clients[client]
		if !exists || binding.Route == "" || destination.Endpoint == "" {
			return Config{}, newLoadError(LoadPhaseValidate, fmt.Errorf("client forwarding %q requires an existing selected binding and endpoint", client))
		}
		runtime, err := cfg.ResolveRuntime(client, "")
		if err != nil || runtime.CredentialProjectionFingerprint(client) != destination.UpstreamIdentity || runtime.Protocol != destination.Protocol {
			return Config{}, newLoadError(LoadPhaseValidate, fmt.Errorf("client forwarding %q does not match the selected upstream identity", client))
		}
		binding.ForwardingEndpoint = destination.Endpoint
		cfg.Clients[client] = binding
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, newLoadError(LoadPhaseValidate, err)
	}
	return cfg, nil
}

func configurationsEqual(left, right Config) (bool, error) {
	leftData, leftErr := encodeConfig(left)
	rightData, rightErr := encodeConfig(right)
	if err := errors.Join(leftErr, rightErr); err != nil {
		return false, err
	}
	leftForwarding, leftErr := encodeForwarding(left)
	rightForwarding, rightErr := encodeForwarding(right)
	return bytes.Equal(leftData, rightData) && bytes.Equal(leftForwarding, rightForwarding), errors.Join(leftErr, rightErr)
}
