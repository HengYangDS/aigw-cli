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

// Activation separates the prerequisites for local client use and caches
// credential availability metadata, never Token values or inference health.
// An empty scope is deferred despite a valid catalogue.
type Activation struct {
	EnabledClients                int
	State                         domainreadiness.State
	CapabilityPrerequisite        string
	SelectionPrerequisite         string
	CredentialPrerequisite        string
	ClientCredentialPrerequisites map[string]string
	ProjectionPrerequisites       map[string]string
	VerificationPrerequisite      string
	observedCredentials           map[string]credentialObservation
}

type credentialObservation struct {
	available bool
	err       error
}

// CredentialAvailability reuses metadata already observed for one selected
// Account; it never returns or reads the Token value.
func (a *Activation) CredentialAvailability(account string) (available bool, err error, observed bool) {
	if a == nil {
		return false, nil, false
	}
	result, observed := a.observedCredentials[account]
	return result.available, result.err, observed
}

// NextActionFor selects one actionable continuation from activation metadata
// and any additionally observed client states.
func (a *Activation) NextActionFor(clients []domainreadiness.Client) string {
	if a == nil {
		return ""
	}
	if a.CapabilityPrerequisite != "" {
		return a.CapabilityPrerequisite
	}
	if a.CredentialPrerequisite != "" {
		return a.CredentialPrerequisite
	}
	if a.SelectionPrerequisite != "" {
		return a.SelectionPrerequisite
	}
	for _, observation := range a.observedCredentials {
		if observation.err != nil {
			return "aigw doctor"
		}
	}
	projectionAction := ""
	for _, spec := range configuration.AdmittedClientSpecs() {
		if action := a.ProjectionPrerequisites[spec.ID]; action != "" && a.ClientCredentialPrerequisites[spec.ID] == "" {
			projectionAction = action
			break
		}
	}
	if a.EnabledClients == 0 && projectionAction != "" {
		return projectionAction
	}
	if action := observedClientAction(clients); action != "" {
		return action
	}
	if projectionAction != "" {
		return projectionAction
	}
	for _, spec := range configuration.AdmittedClientSpecs() {
		if action := a.ClientCredentialPrerequisites[spec.ID]; action != "" {
			return action
		}
	}
	for _, spec := range configuration.AdmittedClientSpecs() {
		if action := a.ProjectionPrerequisites[spec.ID]; action != "" {
			return action
		}
	}
	return a.VerificationPrerequisite
}

func observedClientAction(clients []domainreadiness.Client) string {
	bestAction := ""
	bestPriority := -1
	attention := false
	for _, client := range clients {
		priority := 0
		switch client.State {
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			priority = 3
			attention = true
		case domainreadiness.Configured, domainreadiness.EndpointChecked, domainreadiness.InferenceChecked:
			priority = 2
		case domainreadiness.Deferred:
			priority = 1
		}
		if client.NextAction != "" && priority > bestPriority {
			bestAction = client.NextAction
			bestPriority = priority
		}
	}
	if attention && bestPriority < 3 {
		return "aigw repair"
	}
	return bestAction
}

func observeCredential(store secrets.Store, observed map[string]credentialObservation, account string) (credentialObservation, bool) {
	if store == nil {
		return credentialObservation{}, false
	}
	observation, exists := observed[account]
	if !exists {
		observation.available, observation.err = store.Exists(account)
		observed[account] = observation
	}
	return observation, true
}

func projectionAction(spec configuration.ClientSpec) string {
	return "Install " + spec.Label + " if needed, then run `aigw sync`"
}

// ProjectionPrerequisites derives pending native-client work from selected
// bindings without reading credential metadata or claiming projection health.
func ProjectionPrerequisites(cfg configuration.Config) map[string]string {
	prerequisites := map[string]string{}
	for _, spec := range configuration.AdmittedClientSpecs() {
		binding := cfg.Clients[spec.ID]
		if !binding.Enabled || binding.Executable != "" {
			continue
		}
		prerequisites[spec.ID] = projectionAction(spec)
	}
	return prerequisites
}

func assessEnabledProjection(cfg configuration.Config, store secrets.Store, result Activation) Activation {
	result.ProjectionPrerequisites = ProjectionPrerequisites(cfg)
	result.ClientCredentialPrerequisites = map[string]string{}
	result.observedCredentials = map[string]credentialObservation{}
	blocked := 0
	metadataUnavailable := false
	for _, spec := range configuration.AdmittedClientSpecs() {
		if !cfg.Clients[spec.ID].Enabled {
			continue
		}
		runtime, err := cfg.ResolveRuntime(spec.ID, "")
		if err == nil && runtime.UsesAIGWCredentialStore() {
			observation, observed := observeCredential(store, result.observedCredentials, runtime.AccountID)
			switch {
			case !observed || observation.err != nil:
				metadataUnavailable = true
			case !observation.available:
				result.ClientCredentialPrerequisites[spec.ID], _ = credential.TokenRecovery(store, runtime.AccountID)
			}
		}
		if result.ProjectionPrerequisites[spec.ID] != "" || result.ClientCredentialPrerequisites[spec.ID] != "" {
			blocked++
		}
	}
	if metadataUnavailable {
		result.State = domainreadiness.Unavailable
		result.CredentialPrerequisite = "aigw doctor"
	} else if len(result.ProjectionPrerequisites) == result.EnabledClients {
		result.State = domainreadiness.Deferred
	} else if blocked == 0 {
		result.VerificationPrerequisite = "aigw check"
	}
	return result
}

type inactiveClientAssessment struct {
	cfg            configuration.Config
	store          secrets.Store
	result         Activation
	readOnly       bool
	seen           map[string]bool
	alternatives   []string
	firstMissing   string
	backendMissing bool
}

func (assessment *inactiveClientAssessment) consider(client, route string, selected bool) bool {
	if route == "" {
		return false
	}
	candidate, err := assessment.cfg.ResolveRuntime(client, route)
	if err != nil {
		return false
	}
	spec, _ := configuration.ClientSpecFor(client)
	assessment.result.ProjectionPrerequisites[client] = projectionAction(spec)
	if !candidate.UsesAIGWCredentialStore() {
		delete(assessment.result.ClientCredentialPrerequisites, client)
		return true
	}
	if assessment.store == nil {
		assessment.result.ClientCredentialPrerequisites[client] = "aigw doctor"
		assessment.backendMissing = true
		return false
	}
	if !selected && !assessment.readOnly {
		assessment.result.ClientCredentialPrerequisites[client], _ = credential.TokenRecovery(assessment.store, candidate.AccountID)
		if !assessment.seen[candidate.AccountID] {
			assessment.alternatives = append(assessment.alternatives, "aigw rotate "+candidate.AccountID)
			assessment.seen[candidate.AccountID] = true
		}
		return false
	}
	observation, _ := observeCredential(assessment.store, assessment.result.observedCredentials, candidate.AccountID)
	if observation.err != nil {
		assessment.result.State = domainreadiness.Unavailable
		assessment.result.CredentialPrerequisite = "aigw doctor"
		return true
	}
	if observation.available {
		delete(assessment.result.ClientCredentialPrerequisites, client)
		return true
	}
	action, _ := credential.TokenRecovery(assessment.store, candidate.AccountID)
	assessment.result.ClientCredentialPrerequisites[client] = action
	if selected && assessment.firstMissing == "" {
		assessment.firstMissing = action
	} else if !selected && !assessment.seen[candidate.AccountID] {
		assessment.alternatives = append(assessment.alternatives, secrets.EnvironmentKey(candidate.AccountID))
		assessment.seen[candidate.AccountID] = true
	}
	return false
}

func (assessment *inactiveClientAssessment) finish() Activation {
	switch {
	case assessment.firstMissing != "":
		assessment.result.CredentialPrerequisite = assessment.firstMissing
	case len(assessment.alternatives) > 0:
		slices.Sort(assessment.alternatives)
		prefix := "Choose one compatible Account: "
		if assessment.readOnly {
			prefix = "Set one compatible Account variable: "
		}
		assessment.result.CredentialPrerequisite = prefix + strings.Join(assessment.alternatives, " or ")
	case assessment.backendMissing:
		assessment.result.State = domainreadiness.Unavailable
		assessment.result.CredentialPrerequisite = "aigw doctor"
	default:
		assessment.result.SelectionPrerequisite = "aigw use --help"
	}
	return assessment.result
}

func assessInactiveClients(cfg configuration.Config, store secrets.Store, result Activation) Activation {
	result.observedCredentials = map[string]credentialObservation{}
	result.ClientCredentialPrerequisites = map[string]string{}
	result.ProjectionPrerequisites = map[string]string{}
	assessment := inactiveClientAssessment{
		cfg: cfg, store: store, result: result, readOnly: secrets.IsReadOnly(store), seen: map[string]bool{},
	}
	for _, client := range configuration.AdmittedClientIDs() {
		if assessment.consider(client, cfg.SelectedRoute(client), true) {
			return assessment.result
		}
	}
	for _, client := range configuration.AdmittedClientIDs() {
		if cfg.SelectedRoute(client) != "" {
			continue
		}
		recommendation := cfg.Recommendations[client]
		if assessment.consider(client, recommendation.Primary.Route, false) {
			return assessment.result
		}
		for _, alternative := range recommendation.Alternatives {
			if assessment.consider(client, alternative.Route, false) {
				return assessment.result
			}
		}
	}
	return assessment.finish()
}

// NextActionAfterAccountConnection derives the next explicit client step after
// a successful Token write. It does not inspect another Account's credential,
// select a Route, or mutate client configuration.
func NextActionAfterAccountConnection(cfg configuration.Config, accountID string) string {
	for _, spec := range configuration.AdmittedClientSpecs() {
		runtime, resolveErr := cfg.ResolveRuntime(spec.ID, "")
		if resolveErr != nil || runtime.AccountID != accountID {
			continue
		}
		binding := cfg.Clients[spec.ID]
		if !binding.Enabled || binding.Executable == "" {
			return projectionAction(spec)
		}
		return "aigw check"
	}
	prospective, err := cfg.SelectRoutesForConnectedAccounts([]string{accountID})
	if err != nil {
		return "aigw status"
	}
	for _, spec := range configuration.AdmittedClientSpecs() {
		if cfg.SelectedRoute(spec.ID) != "" {
			continue
		}
		binding := prospective.Clients[spec.ID]
		route := prospective.Routes[binding.Route]
		if binding.Route == "" || route.Account != accountID {
			continue
		}
		action := "aigw use --for " + spec.ID
		if len(route.Interfaces) > 1 {
			action += " --protocol " + string(binding.Protocol)
		}
		return action + " " + binding.Route
	}
	return "aigw use --help"
}

// AssessActivation selects a safe continuation without observing unselected
// native credentials. Environment credential metadata is read only when that
// backend is explicitly selected.
func AssessActivation(cfg configuration.Config, store secrets.Store) Activation {
	result := Activation{EnabledClients: len(cfg.EnabledClientIDs())}
	if result.EnabledClients != 0 {
		return assessEnabledProjection(cfg, store, result)
	}
	result.State = domainreadiness.Deferred
	if len(cfg.Routes) == 0 {
		result.CapabilityPrerequisite = "aigw setup"
		return result
	}
	return assessInactiveClients(cfg, store, result)
}
