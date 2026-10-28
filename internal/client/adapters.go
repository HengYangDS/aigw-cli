package client

import "aigw-cli/internal/discovery"

// NewHermesAdapter returns the Hermes adapter for the composition root to register.
func NewHermesAdapter() Adapter { return hermesAdapter{} }

// NewDiscoverer binds an adapter registry to one host observation source.
func NewDiscoverer(registry Registry, source DiscoverySource) discovery.Discoverer {
	return registeredDiscoverer{registry: registry, source: source}
}

type registeredDiscoverer struct {
	registry Registry
	source   DiscoverySource
}

func (discoverer registeredDiscoverer) Discover() discovery.Result {
	return discoverer.registry.Discover(discoverer.source)
}
