package domain

import (
	"fmt"
	"math"
)

// RetryPolicy declares native budgets for one client connection. Nil fields
// leave the client's defaults unchanged; zero is an explicit retry prohibition.
type RetryPolicy struct {
	RequestMaxRetries *uint64 `json:"request_max_retries,omitempty"`
	StreamMaxRetries  *uint64 `json:"stream_max_retries,omitempty"`
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

func (policy *RetryPolicy) validate(client string, spec ClientSpec) error {
	if policy == nil {
		return nil
	}
	if !spec.RetryBudgets {
		return fmt.Errorf("client %q has no admitted native retry-budget capability", client)
	}
	if policy.RequestMaxRetries == nil && policy.StreamMaxRetries == nil {
		return fmt.Errorf("client %q retries need a request or stream budget; omit retries for the client's defaults", client)
	}
	for _, budget := range []*uint64{policy.RequestMaxRetries, policy.StreamMaxRetries} {
		if budget != nil && *budget > math.MaxInt64 {
			return fmt.Errorf("client %q retry budget exceeds TOML's maximum integer %d", client, int64(math.MaxInt64))
		}
	}
	return nil
}

func (c *Config) retryPolicyFor(client string, selection ClientSelection, upstream string, protocol EndpointProtocol) *RetryPolicy {
	binding := c.Clients[client]
	bound, exists := c.Models[binding.Model]
	selected := c.Models[selection.Model]
	spec, _ := c.admission.Spec(client)
	if !exists || bound.Provider != selected.Provider || binding.RetryPolicy == nil ||
		selectedModelProvider(spec, binding.selection()) != selectedModelProvider(spec, selection) ||
		selectedAuthentication(binding.selection()) != selectedAuthentication(selection) {
		return nil
	}
	endpoint, boundProtocol, err := spec.resolveModelEndpoint(c.Providers[bound.Provider], bound, binding.Protocol)
	if err != nil || endpoint != upstream || boundProtocol != protocol {
		return nil
	}
	return binding.RetryPolicy.Clone()
}
