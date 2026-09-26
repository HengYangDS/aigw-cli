// Package activation classifies the enabled-client scope of local control-plane operations.
package activation

import (
	"slices"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
)

// Activation describes enabled-client scope and the next prerequisite, not
// endpoint or inference health. An empty scope is deferred despite a valid catalogue.
type Activation struct {
	EnabledClients         int
	State                  domainreadiness.State
	NextAction             string
	CredentialPrerequisite bool
}

// AssessActivation selects a safe continuation without observing unselected
// native credentials. Environment credential metadata is read only when that
// backend is explicitly selected.
func AssessActivation(cfg configuration.Config, store secrets.Store) Activation {
	result := Activation{EnabledClients: len(cfg.EnabledClientIDs())}
	if result.EnabledClients != 0 {
		return result
	}
	result.State = domainreadiness.Deferred
	if len(cfg.Routes) == 0 {
		result.NextAction = "aigw setup"
		return result
	}
	if !secrets.IsReadOnly(store) {
		result.NextAction = "aigw sync"
		return result
	}

	firstMissing := ""
	var alternatives []string
	seen := map[string]bool{}
	consider := func(client, route string, selected bool) bool {
		if route == "" {
			return false
		}
		candidate, err := cfg.ResolveRuntime(client, route)
		if err != nil {
			return false
		}
		if !candidate.RequiresAccountToken() {
			result.NextAction = "aigw sync"
			return true
		}
		if seen[candidate.AccountID] {
			return false
		}
		seen[candidate.AccountID] = true
		available, err := store.Exists(candidate.AccountID)
		if err != nil {
			result.State = domainreadiness.Unavailable
			result.NextAction = "aigw doctor"
			return true
		}
		if available {
			result.NextAction = "aigw sync"
			return true
		}
		if selected && firstMissing == "" {
			firstMissing, _ = credential.TokenRecovery(store, candidate.AccountID)
		} else if !selected {
			alternatives = append(alternatives, secrets.EnvironmentKey(candidate.AccountID))
		}
		return false
	}
	for _, client := range configuration.AdmittedClientIDs() {
		if consider(client, cfg.SelectedRoute(client), true) {
			return result
		}
	}
	for _, client := range configuration.AdmittedClientIDs() {
		if cfg.SelectedRoute(client) != "" {
			continue
		}
		recommendation := cfg.Recommendations[client]
		if consider(client, recommendation.Primary.Route, false) {
			return result
		}
		for _, alternative := range recommendation.Alternatives {
			if consider(client, alternative.Route, false) {
				return result
			}
		}
	}
	if firstMissing != "" {
		result.NextAction = firstMissing
		result.CredentialPrerequisite = true
	} else if len(alternatives) > 0 {
		slices.Sort(alternatives)
		result.NextAction = "Set one compatible Account variable: " + strings.Join(alternatives, " or ")
		result.CredentialPrerequisite = true
	} else {
		result.NextAction = "aigw use --help"
	}
	return result
}
