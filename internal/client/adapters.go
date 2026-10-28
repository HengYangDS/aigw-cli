package client

import "aigw-cli/internal/discovery"

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
