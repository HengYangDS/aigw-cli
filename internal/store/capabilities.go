package store

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"github.com/pelletier/go-toml/v2"
)

// routeCapabilities is the Store's component for Route capabilities outside
// the vocabulary published with schema six. Published credential readers
// validate that vocabulary strictly and never read this component.
type routeCapabilities struct {
	Routes map[string]map[domain.EndpointProtocol]recordedCapabilities `toml:"routes"`
}

// recordedCapabilities applies only while its Route keeps the identity it was
// recorded for.
type recordedCapabilities struct {
	Identity     string              `toml:"identity"`
	Capabilities []domain.Capability `toml:"capabilities"`
}

func (s Store) capabilitiesPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".capabilities.toml"
}

// publishedCapability reports whether every published schema-six reader
// accepts capability in the canonical configuration. That vocabulary is fixed:
// a capability added later belongs with tool_search.
func publishedCapability(capability domain.Capability) bool {
	switch capability {
	case domain.CapabilityText, domain.CapabilityReasoning, domain.CapabilityStreaming, domain.CapabilityTools,
		domain.CapabilityStructuredOutput, domain.CapabilityContinuation, domain.CapabilityCompaction,
		domain.CapabilityMultimodalInput:
		return true
	case domain.CapabilityToolSearch:
	}
	return false
}

// capabilityIdentity hashes the Account, endpoint, upstream model and
// interface that a recorded capability qualified.
func capabilityIdentity(cfg domain.Config, route domain.Route, protocol domain.EndpointProtocol) string {
	endpoint := cfg.Accounts[route.Account].Endpoints.For(protocol)
	identity := route.Account + "\x00" + endpoint + "\x00" + route.UpstreamModelID() + "\x00" + string(protocol)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
}

// keepPublishedCapabilities removes later capabilities from Routes that the
// caller owns, leaving the form every published schema-six reader accepts.
func keepPublishedCapabilities(cfg domain.Config) {
	for _, route := range cfg.Routes {
		for protocol, capabilities := range route.Interfaces {
			route.Interfaces[protocol] = slices.DeleteFunc(capabilities, func(capability domain.Capability) bool {
				return !publishedCapability(capability)
			})
		}
	}
}

func encodeRouteCapabilities(cfg domain.Config) ([]byte, error) {
	recorded := routeCapabilities{Routes: map[string]map[domain.EndpointProtocol]recordedCapabilities{}}
	for id, route := range cfg.Routes {
		for protocol, capabilities := range route.Interfaces {
			later := slices.DeleteFunc(slices.Clone(capabilities), publishedCapability)
			if len(later) == 0 {
				continue
			}
			if recorded.Routes[id] == nil {
				recorded.Routes[id] = map[domain.EndpointProtocol]recordedCapabilities{}
			}
			recorded.Routes[id][protocol] = recordedCapabilities{Identity: capabilityIdentity(cfg, route, protocol), Capabilities: later}
		}
	}
	if len(recorded.Routes) == 0 {
		return nil, nil
	}
	data, err := toml.Marshal(recorded)
	return separateTOMLTableBlocks(data), err
}

// decodeRouteCapabilities adds each recorded capability to its Route while the
// Route still matches the recorded identity; other entries no longer apply.
func decodeRouteCapabilities(cfg domain.Config, snapshot platform.FileSnapshot) (domain.Config, error) {
	if !snapshot.Exists {
		return cfg, nil
	}
	var recorded routeCapabilities
	decoder := toml.NewDecoder(bytes.NewReader(snapshot.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&recorded); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseParse, fmt.Errorf("parse route capabilities: %w", err))
	}
	if len(recorded.Routes) == 0 {
		return domain.Config{}, newLoadError(LoadPhaseValidate, errors.New("route capability component has no entries"))
	}
	for _, id := range slices.Sorted(maps.Keys(recorded.Routes)) {
		for _, protocol := range slices.Sorted(maps.Keys(recorded.Routes[id])) {
			entry := recorded.Routes[id][protocol]
			if err := entry.validate(id, protocol); err != nil {
				return domain.Config{}, newLoadError(LoadPhaseValidate, err)
			}
			route, exists := cfg.Routes[id]
			capabilities, admitted := route.Interfaces[protocol]
			if !exists || !admitted || entry.Identity != capabilityIdentity(cfg, route, protocol) {
				continue
			}
			for _, capability := range entry.Capabilities {
				if !slices.Contains(capabilities, capability) {
					capabilities = append(capabilities, capability)
				}
			}
			route.Interfaces[protocol] = capabilities
		}
	}
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseValidate, err)
	}
	return cfg, nil
}

func (entry recordedCapabilities) validate(route string, protocol domain.EndpointProtocol) error {
	if len(entry.Capabilities) == 0 {
		return fmt.Errorf("route %q interface %q records no capability", route, protocol)
	}
	for index, capability := range entry.Capabilities {
		if !capability.Known() {
			return fmt.Errorf("route %q interface %q has unknown capability %q", route, protocol, capability)
		}
		if publishedCapability(capability) {
			return fmt.Errorf("route %q interface %q records %q, which belongs in the canonical configuration", route, protocol, capability)
		}
		if slices.Contains(entry.Capabilities[:index], capability) {
			return fmt.Errorf("route %q interface %q repeats capability %q", route, protocol, capability)
		}
	}
	return nil
}
