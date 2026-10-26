package configuration

import (
	"fmt"
	"math"
)

// RetryPolicy declares native budgets for one client connection. Nil fields
// leave the client's defaults unchanged; zero is an explicit retry prohibition.
type RetryPolicy struct {
	RequestMaxRetries *uint64 `json:"request_max_retries,omitempty" toml:"request_max_retries,omitempty"`
	StreamMaxRetries  *uint64 `json:"stream_max_retries,omitempty" toml:"stream_max_retries,omitempty"`
}

// Clone returns an independent policy, preserving absence and explicit zero.
func (policy *RetryPolicy) Clone() *RetryPolicy {
	if policy == nil {
		return nil
	}
	cloned := *policy
	if policy.RequestMaxRetries != nil {
		value := *policy.RequestMaxRetries
		cloned.RequestMaxRetries = &value
	}
	if policy.StreamMaxRetries != nil {
		value := *policy.StreamMaxRetries
		cloned.StreamMaxRetries = &value
	}
	return &cloned
}

func (policy *RetryPolicy) validate(client string) error {
	if policy == nil {
		return nil
	}
	if client != ClientCodex {
		return fmt.Errorf("client %q has no admitted native retry-budget capability", client)
	}
	if policy.RequestMaxRetries == nil && policy.StreamMaxRetries == nil {
		return fmt.Errorf("client %q retry_policy requires at least one native budget; omit the table for native defaults", client)
	}
	for _, budget := range []*uint64{policy.RequestMaxRetries, policy.StreamMaxRetries} {
		if budget != nil && *budget > math.MaxInt64 {
			return fmt.Errorf("client %q retry budget exceeds TOML's maximum integer %d", client, int64(math.MaxInt64))
		}
	}
	return nil
}

// ConnectionChange records explicit destination and retry intent for selection.
// Nil values retain the selected connection; NativeRetries removes its policy.
type ConnectionChange struct {
	ForwardingEndpoint *string
	RetryPolicy        *RetryPolicy
	NativeRetries      bool
}

func (c *Config) retryPolicyFor(client string, selection ClientSelection, upstream string, protocol EndpointProtocol) *RetryPolicy {
	binding := c.Clients[client]
	bound, exists := c.Routes[binding.Route]
	selected := c.Routes[selection.Route]
	if !exists || bound.Account != selected.Account || binding.RetryPolicy == nil ||
		selectedModelProvider(client, binding.selection()) != selectedModelProvider(client, selection) ||
		selectedAuthentication(binding.selection()) != selectedAuthentication(selection) {
		return nil
	}
	spec, _ := ClientSpecFor(client)
	endpoint, boundProtocol, err := spec.ResolveRouteEndpoint(c.Accounts[bound.Account], bound, binding.Protocol)
	if err != nil || endpoint != upstream || boundProtocol != protocol {
		return nil
	}
	return binding.RetryPolicy.Clone()
}
