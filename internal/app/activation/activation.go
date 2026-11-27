// Package activation classifies the enabled-client scope of local control-plane operations.
package activation

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// Activation separates the prerequisites for local client use and caches
// credential availability metadata, never Token values or inference health.
// An empty scope is deferred despite a valid catalog. Obtain it from
// AssessActivation: a value built any other way admits no client, so
// NextActionFor ignores its per-client prerequisites.
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
	// admission orders the per-client prerequisites; AssessActivation takes it
	// from the assessed configuration.
	admission domain.Admission
}

type credentialObservation struct {
	available bool
	err       error
	// refusal is the refusal of the Token's Keychain item when macOS refuses
	// AIGW that item, which leaves the provider without a Token that AIGW can
	// read, as a missing one does.
	refusal *secret.KeychainItemRefusedError
}

// recovery is the step that gives the observed provider a Token that AIGW can
// read: the fix of the Keychain item that macOS refuses AIGW, or else the
// store's own recovery of a missing Token.
func (observation credentialObservation) recovery(store secret.Store, account string) string {
	if observation.refusal != nil {
		return observation.refusal.Fix()
	}
	action, _ := secret.TokenRecovery(store, account)
	return action
}

// CredentialAvailability reuses metadata already observed for one selected
// Account; it never returns or reads the Token value. A Keychain item that
// macOS refuses AIGW is no available Token, and its refusal is the error.
func (a *Activation) CredentialAvailability(account string) (available bool, err error, observed bool) {
	if a == nil {
		return false, nil, false
	}
	result, observed := a.observedCredentials[account]
	if result.refusal != nil {
		return false, result.refusal, observed
	}
	return result.available, result.err, observed
}

// UnobservedCredential returns the failure of a store that could not tell
// whether the Token of an enabled client's provider is there, from the first
// such provider by ID, or nil when every observation could tell. Such a
// client waits for its Token, as for an absent one.
func (a *Activation) UnobservedCredential() error {
	if a == nil {
		return nil
	}
	for _, account := range slices.Sorted(maps.Keys(a.observedCredentials)) {
		if err := a.observedCredentials[account].err; err != nil {
			return err
		}
	}
	return nil
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
		// A store that a blocked path keeps from being observed names the fix
		// that a client's observation rendered for it.
		if action := unobservedAction(clients); action != domainreadiness.CredentialBackendRecovery {
			return action
		}
		return a.CredentialPrerequisite
	}
	if a.SelectionPrerequisite != "" {
		return a.SelectionPrerequisite
	}
	for _, observation := range a.observedCredentials {
		if observation.err != nil {
			return unobservedAction(clients)
		}
	}
	projectionAction := ""
	for _, client := range a.admission.IDs() {
		if action := a.ProjectionPrerequisites[client]; action != "" && a.ClientCredentialPrerequisites[client] == "" {
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
	for _, client := range a.admission.IDs() {
		if action := a.ClientCredentialPrerequisites[client]; action != "" {
			return action
		}
	}
	for _, client := range a.admission.IDs() {
		if action := a.ProjectionPrerequisites[client]; action != "" {
			return action
		}
	}
	return a.VerificationPrerequisite
}

// unobservedAction is the step of a credential store that cannot be observed:
// the one that an unavailable client's observation named for itself, or else
// the local check, which inspects the backend.
func unobservedAction(clients []domainreadiness.Client) string {
	for _, client := range clients {
		if client.State == domainreadiness.Unavailable && client.NextAction != "" {
			return client.NextAction
		}
	}
	return domainreadiness.CredentialBackendRecovery
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
		case domainreadiness.Configured:
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
		return "aigw sync"
	}
	return bestAction
}

func observeCredential(store secret.Store, observed map[string]credentialObservation, account string) (credentialObservation, bool) {
	if store == nil {
		return credentialObservation{}, false
	}
	observation, exists := observed[account]
	if !exists {
		// A slot that holds no value holds no Token, and neither does an item
		// that macOS refuses AIGW.
		token := secret.ObserveToken(store, account)
		observation = credentialObservation{available: token.Usable(), err: token.Unobserved, refusal: token.Refusal}
		observed[account] = observation
	}
	return observation, true
}

// syncStep is the step of files that a sync writes.
const syncStep = "aigw sync"

// recorded ends the problem of a client whose files match AIGW's record.
const recorded = "; its files match AIGW's record"

// Awaited is what an enabled client waits for before AIGW keeps its files, as
// AwaitedFiles decides it. Its zero value waits for nothing: the client's
// inspection, which judges its files against AIGW's record, decides alone.
type Awaited struct {
	// Problem says why the client waits, and Step names the step that gives
	// it its program: a sync, which adopts the program that discovery finds,
	// or else the step of adapter.ProgramNotFound.
	Problem, Step string
	// Found marks a client whose program discovery finds.
	Found bool
	// Unwritten marks files that do not match AIGW's record, as those that
	// AIGW never wrote do not, and that a sync writes once the client has its
	// program: they count as not projected.
	Unwritten bool
}

// AwaitedFiles decides, for status and the local check alike, what an enabled
// client waits for from what AIGW observed of it: inspected, its inspection,
// which judges its files against AIGW's record, and found, which tells
// whether discovery finds its program and is asked only when the answer
// matters. Whether the files count as projected follows from the enabled
// state, the targets and AIGW's record, never from whether the configuration
// holds a program path: the inspection decides alone for a client that
// declares its program, by path or by extensions directory, and reports one
// that its program stops as stopped, and for a client whose inspection names
// a step of its own, such as for a file edited outside AIGW. A client that
// declares no program and whose files either match AIGW's record or wait for
// a sync waits for the step that gives it a program.
func AwaitedFiles(cfg domain.Config, clientID string, inspected adapter.Status, found func() bool) Awaited {
	binding := cfg.Clients[clientID]
	// An inspection that names no step of its own names a sync, as readiness
	// reads it too.
	unwritten := !inspected.Ready && cmp.Or(inspected.RepairAction, syncStep) == syncStep
	if !binding.Enabled || binding.DeclaresProgram() || !inspected.Ready && !unwritten {
		return Awaited{}
	}
	awaited := Awaited{Found: found(), Unwritten: unwritten}
	spec, _ := cfg.Admission().Spec(clientID)
	switch {
	case !awaited.Found:
		stop := adapter.ProgramNotFound(cfg, clientID)
		awaited.Problem, awaited.Step = stop.Error(), stop.NextAction()
		if !unwritten {
			awaited.Problem += recorded
		}
	case unwritten:
		awaited.Problem, awaited.Step = spec.Label+" is installed, but AIGW has not written its files yet", syncStep
	default:
		awaited.Problem, awaited.Step = cfg.Admission().ProductName(clientID)+" declares no program, and a sync adopts the one that discovery finds"+recorded, syncStep
	}
	return awaited
}

// enableAction resumes a disabled client binding that keeps its selection;
// synchronization skips it. Enabling does not depend on the stored protocol,
// which selecting the model again would have to equal. Enabling keeps the
// program that the client declares, or adopts the one that discovery finds,
// and asks for a configuration file when the client reads one, so the step
// names that file for the operator to choose.
func enableAction(spec domain.ClientSpec) string {
	action := "aigw client enable " + spec.ID
	if spec.TargetFile != "" {
		action += " --target <" + spec.TargetFile + ">"
	}
	return action
}

// ProjectionPrerequisites derives pending native-client work from the
// configuration alone, without reading credential metadata, observing a
// client or claiming projection health: an enabled client that declares no
// program has not been converged since discovery last found it, since
// convergence adopts the program that discovery finds, so it waits for the
// step of adapter.ProgramNotFound, and so are its files taken to. A command
// that observes the clients replaces this estimate with what AwaitedFiles
// decides (see ObserveFiles). Whether the files of a client that declares a
// program are projected is its inspection's to tell, against AIGW's record,
// and a declared program that cannot be used stops the client instead.
func ProjectionPrerequisites(cfg domain.Config) map[string]string {
	prerequisites := map[string]string{}
	for _, spec := range cfg.Admission().Specs() {
		binding := cfg.Clients[spec.ID]
		if !binding.Enabled || binding.DeclaresProgram() {
			continue
		}
		prerequisites[spec.ID] = adapter.ProgramNotFound(cfg, spec.ID).NextAction()
	}
	return prerequisites
}

// ObserveFiles replaces the estimate of what the enabled clients wait for,
// which AssessActivation makes from the configuration alone, with what an
// observation of them found, as AwaitedFiles decides it for each client. The
// state and the verification step follow the observation: the activation is
// deferred while every enabled client's files are unwritten, not while a
// client whose files match AIGW's record waits for its program.
func (a *Activation) ObserveFiles(observed map[string]Awaited) {
	if a == nil || a.EnabledClients == 0 {
		return
	}
	a.ProjectionPrerequisites = map[string]string{}
	unwritten := 0
	for client, awaited := range observed {
		if awaited.Step == "" {
			continue
		}
		a.ProjectionPrerequisites[client] = awaited.Step
		if awaited.Unwritten {
			unwritten++
		}
	}
	a.settleEnabled(unwritten)
}

// settleEnabled derives the state of an activation with enabled clients from
// their prerequisites, of which unwritten clients' files are not written:
// unavailable credential metadata stays so; when no enabled client's files
// are written, the activation is deferred; when no enabled client waits for
// anything, the next step verifies them.
func (a *Activation) settleEnabled(unwritten int) {
	if a.State == domainreadiness.Unavailable {
		return
	}
	a.State, a.VerificationPrerequisite = "", ""
	blocked := 0
	for _, client := range a.admission.IDs() {
		if a.ProjectionPrerequisites[client] != "" || a.ClientCredentialPrerequisites[client] != "" {
			blocked++
		}
	}
	switch {
	case unwritten == a.EnabledClients:
		a.State = domainreadiness.Deferred
	case blocked == 0:
		a.VerificationPrerequisite = "aigw check"
	}
}

func assessEnabledProjection(cfg domain.Config, store secret.Store, result Activation) Activation {
	result.ProjectionPrerequisites = ProjectionPrerequisites(cfg)
	result.ClientCredentialPrerequisites = map[string]string{}
	result.observedCredentials = map[string]credentialObservation{}
	for _, spec := range cfg.Admission().Specs() {
		if !cfg.Clients[spec.ID].Enabled {
			continue
		}
		runtime, err := cfg.ResolveRuntime(spec.ID, domain.ModelRef{})
		if err != nil || !runtime.UsesAIGWCredentialStore() {
			continue
		}
		observation, observed := observeCredential(store, result.observedCredentials, runtime.ProviderID)
		switch {
		case !observed || observation.err != nil:
			result.State = domainreadiness.Unavailable
			result.CredentialPrerequisite = "aigw check --depth local"
		case !observation.available:
			result.ClientCredentialPrerequisites[spec.ID] = observation.recovery(store, runtime.ProviderID)
		}
	}
	result.settleEnabled(len(result.ProjectionPrerequisites))
	return result
}

type inactiveClientAssessment struct {
	cfg          domain.Config
	store        secret.Store
	result       Activation
	readOnly     bool
	seen         map[string]bool
	alternatives []string
	// refused are the refusals of the Keychain items of recommended
	// providers that macOS refuses AIGW, one for each provider.
	refused        []*secret.KeychainItemRefusedError
	firstMissing   string
	backendMissing bool
}

func (assessment *inactiveClientAssessment) consider(client string, ref domain.ModelRef, selected bool) bool {
	if ref.IsZero() {
		return false
	}
	candidate, err := assessment.cfg.ResolveRuntime(client, ref)
	if err != nil {
		return false
	}
	if selected {
		// Every binding is disabled here; synchronization skips a disabled selection.
		spec, _ := assessment.cfg.Admission().Spec(client)
		assessment.result.ProjectionPrerequisites[client] = enableAction(spec)
	} else {
		// A sync enables the recommended client and adopts the program that
		// discovery finds; without one, it waits as an enabled client does.
		assessment.result.ProjectionPrerequisites[client] = adapter.ProgramNotFound(assessment.cfg, client).NextAction()
	}
	if !candidate.UsesAIGWCredentialStore() {
		delete(assessment.result.ClientCredentialPrerequisites, client)
		return true
	}
	if assessment.store == nil {
		assessment.result.ClientCredentialPrerequisites[client] = "aigw check --depth local"
		assessment.backendMissing = true
		return false
	}
	observation, _ := observeCredential(assessment.store, assessment.result.observedCredentials, candidate.ProviderID)
	if observation.err != nil {
		assessment.result.State = domainreadiness.Unavailable
		assessment.result.CredentialPrerequisite = "aigw check --depth local"
		return true
	}
	if observation.available {
		delete(assessment.result.ClientCredentialPrerequisites, client)
		if !selected && !assessment.readOnly {
			// The recommendation's provider holds its Token, so applying the
			// team file again selects the model; the file's path is not known
			// here. An environment Token is adopted by a sync instead.
			assessment.result.SelectionPrerequisite = "aigw setup --from <file>"
		}
		return true
	}
	action := observation.recovery(assessment.store, candidate.ProviderID)
	assessment.result.ClientCredentialPrerequisites[client] = action
	if selected && assessment.firstMissing == "" {
		assessment.firstMissing = action
	} else if !selected && !assessment.seen[candidate.ProviderID] {
		assessment.seen[candidate.ProviderID] = true
		switch {
		case observation.refusal != nil:
			// The command that stores a Token would ask for one, send it to
			// the provider and then end at the item, so the item's fix is the
			// step.
			assessment.refused = append(assessment.refused, observation.refusal)
		case assessment.readOnly:
			assessment.alternatives = append(assessment.alternatives, secret.EnvironmentKey(candidate.ProviderID))
		default:
			assessment.alternatives = append(assessment.alternatives, "aigw provider rotate "+candidate.ProviderID)
		}
	}
	return false
}

// choice is the step that gives one recommended provider a Token: the
// commands of the providers without one, joined as a choice, and then, for
// each provider whose Keychain item macOS refuses AIGW, a sentence of its own
// that names the provider and the item's fix in the refusal's words. One
// provider is no choice, so its command or its item's fix stands alone.
func (assessment *inactiveClientAssessment) choice() string {
	slices.Sort(assessment.alternatives)
	slices.SortFunc(assessment.refused, func(left, right *secret.KeychainItemRefusedError) int {
		return strings.Compare(left.Provider, right.Provider)
	})
	if len(assessment.alternatives) == 0 && len(assessment.refused) == 1 {
		return assessment.refused[0].Fix()
	}
	sentences := make([]string, 0, 1+len(assessment.refused))
	if len(assessment.alternatives) > 0 {
		prefix, continuation := "Choose one compatible provider: ", ""
		if assessment.readOnly {
			// Only synchronization adopts an environment Token that appears later.
			prefix, continuation = "Set one compatible provider's variable: ", ", then run `aigw sync`"
		}
		if len(assessment.alternatives)+len(assessment.refused) == 1 && !assessment.readOnly {
			prefix = ""
		}
		commands := prefix + strings.Join(assessment.alternatives, " or ") + continuation
		if len(assessment.refused) > 0 {
			// The commands end as a sentence where the sentence of an item follows.
			commands += "."
		}
		sentences = append(sentences, commands)
	}
	for _, refusal := range assessment.refused {
		sentences = append(sentences, refusal.Title()+": "+refusal.Fix())
	}
	return strings.Join(sentences, " ")
}

func (assessment *inactiveClientAssessment) finish() Activation {
	switch {
	case assessment.firstMissing != "":
		assessment.result.CredentialPrerequisite = assessment.firstMissing
	case len(assessment.alternatives)+len(assessment.refused) > 0:
		assessment.result.CredentialPrerequisite = assessment.choice()
	case assessment.backendMissing:
		assessment.result.State = domainreadiness.Unavailable
		assessment.result.CredentialPrerequisite = "aigw check --depth local"
	default:
		// No client selects a model, and no recommendation offers one: the
		// user chooses both, which the placeholders leave to them.
		assessment.result.SelectionPrerequisite = "aigw use <client> <provider/model>"
	}
	return assessment.result
}

func assessInactiveClients(cfg domain.Config, store secret.Store, result Activation) Activation {
	result.observedCredentials = map[string]credentialObservation{}
	result.ClientCredentialPrerequisites = map[string]string{}
	result.ProjectionPrerequisites = map[string]string{}
	assessment := inactiveClientAssessment{
		cfg: cfg, store: store, result: result, readOnly: secret.IsReadOnly(store), seen: map[string]bool{},
	}
	for _, client := range cfg.Admission().IDs() {
		if assessment.consider(client, cfg.SelectedModel(client), true) {
			return assessment.result
		}
	}
	for _, client := range cfg.Admission().IDs() {
		if !cfg.SelectedModel(client).IsZero() {
			continue
		}
		recommendation := cfg.Recommendations[client]
		if assessment.consider(client, recommendation.Primary.Model, false) {
			return assessment.result
		}
		for _, alternative := range recommendation.Alternatives {
			if assessment.consider(client, alternative.Model, false) {
				return assessment.result
			}
		}
	}
	return assessment.finish()
}

// NextActionAfterProviderConnection derives the next explicit client step after
// a successful Token write. It does not inspect another Account's credential,
// select a Route, or mutate client configuration.
func NextActionAfterProviderConnection(cfg domain.Config, accountID string) string {
	for _, spec := range cfg.Admission().Specs() {
		runtime, resolveErr := cfg.ResolveRuntime(spec.ID, domain.ModelRef{})
		if resolveErr != nil || runtime.ProviderID != accountID {
			continue
		}
		binding := cfg.Clients[spec.ID]
		switch {
		case !binding.Enabled:
			return enableAction(spec)
		case !binding.DeclaresProgram():
			return adapter.ProgramNotFound(cfg, spec.ID).NextAction()
		}
		return "aigw check"
	}
	prospective, err := cfg.SelectModelsForConnectedProviders([]string{accountID})
	if err != nil {
		return "aigw status"
	}
	for _, spec := range cfg.Admission().Specs() {
		if !cfg.SelectedModel(spec.ID).IsZero() {
			continue
		}
		binding := prospective.Clients[spec.ID]
		model := prospective.Models[binding.Model]
		if binding.Model.IsZero() || model.Provider != accountID {
			continue
		}
		action := "aigw use " + spec.ID + " " + binding.Model.String()
		if len(model.Protocols) > 1 {
			action += " --protocol " + binding.Protocol.Name()
		}
		return action
	}
	return "aigw use --help"
}

// AssessActivation selects a safe continuation without observing unselected
// native credentials. Environment credential metadata is read only when that
// backend is explicitly selected.
func AssessActivation(cfg domain.Config, store secret.Store) Activation {
	result := Activation{EnabledClients: len(cfg.EnabledClientIDs()), admission: cfg.Admission()}
	if result.EnabledClients != 0 {
		return assessEnabledProjection(cfg, store, result)
	}
	result.State = domainreadiness.Deferred
	// Only a configuration without providers needs setup; one whose
	// providers have no model needs a model selected.
	if len(cfg.Providers) == 0 {
		result.CapabilityPrerequisite = "aigw setup"
		return result
	}
	return assessInactiveClients(cfg, store, result)
}
