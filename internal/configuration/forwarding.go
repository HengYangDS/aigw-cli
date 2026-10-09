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

// clientForwarding persists only the Codex destination that immutable schema-six
// readers cannot accept. Store assembles its recorded upstream binding without
// changing Accounts, Routes or credentials.
type clientForwarding struct {
	Clients map[string]forwardingDestination `toml:"clients"`
}

type forwardingDestination struct {
	Endpoint         string           `toml:"endpoint"`
	UpstreamIdentity string           `toml:"upstream_identity"`
	Protocol         EndpointProtocol `toml:"protocol"`
}

// SetForwardingEndpoint binds an explicit Codex destination to its current
// upstream; an empty destination completely withdraws that binding.
func (c *Config) SetForwardingEndpoint(client, endpoint string) error {
	if client != ClientCodex {
		return fmt.Errorf("forwarding destinations are only supported for codex")
	}
	binding, exists := c.Clients[client]
	if !exists || binding.Route == "" {
		return &RuntimeBindingUnselectedError{Client: client}
	}
	binding = binding.withoutForwarding()
	if endpoint != "" {
		if err := validateEndpoint(endpoint); err != nil {
			return fmt.Errorf("client %q forwarding endpoint: %w", client, err)
		}
		runtime, err := c.resolveUpstreamSelection(client, binding.selection())
		if err != nil {
			return err
		}
		binding.ForwardingEndpoint = strings.TrimRight(endpoint, "/")
		binding.ForwardingUpstreamIdentity = runtime.CredentialProjectionFingerprint(client)
		binding.ForwardingProtocol = runtime.Protocol
	}
	c.Clients[client] = binding
	return nil
}

func (binding ClientBinding) withoutForwarding() ClientBinding {
	binding.ForwardingEndpoint = ""
	binding.ForwardingUpstreamIdentity = ""
	binding.ForwardingProtocol = ""
	return binding
}

func (s Store) forwardingPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".forwarding.toml"
}

func encodeForwarding(cfg Config) ([]byte, error) {
	forwarding := clientForwarding{Clients: map[string]forwardingDestination{}}
	for client, binding := range cfg.Clients {
		if err := binding.validateForwarding(client); err != nil {
			return nil, err
		}
		if binding.ForwardingEndpoint != "" {
			if _, err := cfg.ResolveRuntime(client, ""); err != nil {
				return nil, err
			}
			forwarding.Clients[client] = forwardingDestination{
				Endpoint: binding.ForwardingEndpoint, UpstreamIdentity: binding.ForwardingUpstreamIdentity, Protocol: binding.ForwardingProtocol,
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
		binding.ForwardingEndpoint = destination.Endpoint
		binding.ForwardingUpstreamIdentity = destination.UpstreamIdentity
		binding.ForwardingProtocol = destination.Protocol
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

// forwardingFor checks the recorded selected upstream before considering an
// explicit Route; neither resolution nor encoding silently rebinds a destination.
func (c *Config) forwardingFor(client string, runtime Runtime) (string, error) {
	binding := c.Clients[client]
	if err := binding.validateForwarding(client); err != nil {
		return "", err
	}
	if binding.ForwardingEndpoint == "" {
		return "", nil
	}
	selected, err := c.resolveUpstreamSelection(client, binding.selection())
	if err != nil || selected.CredentialProjectionFingerprint(client) != binding.ForwardingUpstreamIdentity || selected.Protocol != binding.ForwardingProtocol {
		return "", fmt.Errorf("client forwarding %q does not match the selected upstream identity", client)
	}
	if !selected.SameUpstream(runtime) {
		return "", nil
	}
	return binding.ForwardingEndpoint, nil
}
