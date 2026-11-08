package check

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"

	"aigw-cli/internal/app"
	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// localCheck is one check of the local depth. A check that does not apply to
// a client is skipped, and its detail says why. A failed check's detail is
// its problem, and it fails the client's local depth in its state, with its
// one fix.
type localCheck struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail"`
	// Target names the client file that the check inspected, relative to the
	// home directory; a file outside the home directory is never named.
	Target     string `json:"target,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
	label      string
	fix        string
	failed     domainreadiness.State
}

const (
	checkPassed  = "passed"
	checkFailed  = "failed"
	checkSkipped = "skipped"
)

func passedCheck(name, label, detail string) localCheck {
	return localCheck{Name: name, label: label, State: checkPassed, Detail: detail}
}

func skippedCheck(name, label, reason string) localCheck {
	return localCheck{Name: name, label: label, State: checkSkipped, Detail: reason}
}

func failedCheck(name, label, problem, fix string, state domainreadiness.State) localCheck {
	return localCheck{Name: name, label: label, State: checkFailed, Detail: problem, fix: fix, failed: state}
}

// conflictingVariables are the environment variables that hand a client a
// credential of their own, which would compete with the Token that AIGW gives
// it.
var conflictingVariables = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "OPENAI_API_KEY"}

// backendFix makes a credential store that AIGW cannot observe available
// before the same command runs again.
const backendFix = "Make the credential store available to this session, such as by unlocking it, then run the same command again."

// sharedChecks are the local checks that every client shares, the
// environment, the configuration, which loaded, and the credential backend,
// with the backend's metadata.
func sharedChecks(deps app.Dependencies) ([]localCheck, secret.BackendSelection) {
	checks := []localCheck{environmentCheck(deps.Env), passedCheck("configuration", "Configuration", "Valid")}
	backend, err := secret.Inspect(deps.Secrets)
	if err != nil {
		return append(checks, failedCheck("credential_backend", "Credential backend", "The credential backend is unavailable", backendFix, domainreadiness.Unavailable)), backend
	}
	access := "read and write"
	if backend.Mutability == "read_only" {
		access = "read only"
	}
	return append(checks, passedCheck("credential_backend", "Credential backend", backend.Kind+", "+access)), backend
}

// environmentCheck observes the variables that compete with AIGW's Token,
// naming them and never their values. One that holds a value fails every
// client's local depth, as it failed doctor's diagnostics; an empty one holds
// no credential.
func environmentCheck(environment []string) localCheck {
	set := map[string]bool{}
	for _, entry := range environment {
		if name, value, found := strings.Cut(entry, "="); found && value != "" && slices.Contains(conflictingVariables, name) {
			set[name] = true
		}
	}
	if len(set) == 0 {
		return passedCheck("environment", "Environment", "No "+joined(conflictingVariables, "or")+" is set")
	}
	names := joined(slices.Sorted(maps.Keys(set)), "and")
	verb := " is set"
	if len(set) > 1 {
		verb = " are set"
	}
	return failedCheck("environment", "Environment",
		names+verb+" in the environment, which a client may send instead of the Token that AIGW gives it",
		"Remove "+names+" from the environment that starts your clients.", domainreadiness.Invalid)
}

// joined lists words for prose: "a", "a or b", "a, b or c".
func joined(words []string, conjunction string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " " + conjunction + " " + words[len(words)-1]
}

// local runs the local depth for one client: its model, the Token's
// presence, read as metadata only, and its client files. The shared checks
// come first, then the client's in that order, and the first that failed
// stops the client, as a Token comes before the files that a sync writes only
// with it.
func (c *checker) local(ctx context.Context, id string) result {
	spec, _ := c.cfg.Admission().Spec(id)
	r := result{spec: spec, enabled: c.cfg.Clients[id].Enabled, tried: c.named.key != ""}
	selected, selectErr := c.cfg.ResolveRuntime(id, "")
	if selectErr == nil {
		r.selection = modelName(c.cfg, selected.ModelID)
	}
	r.runtime, r.resolved = selected, selectErr == nil
	modelErr := selectErr
	if r.tried {
		// The client files still match the client's own selection.
		r.runtime, modelErr = c.cfg.ResolveRuntime(id, c.named.key)
		r.resolved = modelErr == nil
	}
	if r.resolved {
		r.model = modelName(c.cfg, r.runtime.ModelID)
	}
	r.checks = append(r.checks, c.modelCheck(&r, modelErr))
	r.checks = append(r.checks, c.tokenCheck(r))
	r.checks = append(r.checks, c.fileChecks(ctx, &r, selected, selectErr)...)
	environment := c.shared[slices.IndexFunc(c.shared, func(check localCheck) bool { return check.Name == "environment" })]
	for _, check := range append([]localCheck{environment}, r.checks...) {
		if check.State == checkFailed {
			r.fail(failure{depth: depthLocal, state: check.failed, problem: check.Detail, fix: check.fix})
			return r
		}
	}
	r.reached = depthLocal
	return r
}

// modelName names the model stored under key as provider/model.
func modelName(cfg domain.Config, key string) string {
	if model, found := cfg.Models[key]; found {
		return model.Ref().String()
	}
	return key
}

// modelCheck observes whether the client's model resolves to a provider and
// an endpoint for one of its protocols.
func (c *checker) modelCheck(r *result, selectErr error) localCheck {
	name, label := r.spec.ID+":model", r.spec.Label+" model"
	if selectErr == nil {
		detail := r.model + " on " + domain.ProtocolLabel(r.runtime.Protocol)
		if r.tried {
			detail += ", named with --model"
		}
		return passedCheck(name, label, detail)
	}
	if c.cfg.SelectedModel(r.spec.ID) == "" {
		return failedCheck(name, label, "No model is selected", c.selectionFix(r.spec.ID), domainreadiness.Deferred)
	}
	return failedCheck(name, label, r.spec.Label+"'s selected model cannot be resolved", "aigw use "+r.spec.ID+" <provider/model>", domainreadiness.Invalid)
}

// selectionFix is the step for a client without a model: setup without a
// provider, the recommended model, or a model that the user chooses.
func (c *checker) selectionFix(id string) string {
	if len(c.cfg.Providers) == 0 {
		return "aigw setup"
	}
	if model, found := c.cfg.Models[c.cfg.RecommendedModel(id)]; found {
		return "aigw use " + id + " " + model.Ref().String()
	}
	return "aigw use " + id + " <provider/model>"
}

// tokenCheck observes whether the client's provider has a Token, reading its
// metadata only. A client that authenticates itself, or whose Token an
// external helper supplies, needs none from AIGW.
func (c *checker) tokenCheck(r result) localCheck {
	name, label := r.spec.ID+":token", r.spec.Label+" Token"
	switch {
	case !r.resolved:
		return skippedCheck(name, label, "No model resolves, so no provider's Token applies")
	case c.named.token != "":
		return skippedCheck(name, label, "The one-time Token from standard input replaces the stored Token for this check")
	case !r.runtime.RequiresProviderToken():
		return skippedCheck(name, label, c.admission().ProductName(r.spec.ID)+" uses its own authentication, so AIGW reads no Token for it")
	case r.runtime.CredentialCommand != "":
		return skippedCheck(name, label, "An external credential helper gives "+c.admission().ProductName(r.spec.ID)+" its Token")
	}
	provider := r.runtime.ProviderID
	present, err := c.deps.Secrets.Exists(provider)
	switch {
	case err != nil:
		return failedCheck(name, label, "The credential backend cannot tell whether provider "+provider+" has a Token", backendFix, domainreadiness.Unavailable)
	case !present || secret.TokenSlotEmpty(c.deps.Secrets, provider):
		fix, writable := secret.TokenRecovery(c.deps.Secrets, provider)
		if writable {
			fix = "aigw provider rotate " + provider
		} else {
			fix = strings.ToUpper(fix[:1]) + fix[1:] + " to the provider's Token."
		}
		return failedCheck(name, label, "Provider "+provider+" has no Token", fix, domainreadiness.Deferred)
	}
	return passedCheck(name, label, "Provider "+provider+" has a Token")
}

// fileChecks observe whether the client's files match its selection, and
// each file that the client's adapter inspected. A disabled client has none.
func (c *checker) fileChecks(ctx context.Context, r *result, selected domain.Runtime, selectErr error) []localCheck {
	name, label := r.spec.ID+":files", r.spec.Label+" client files"
	binding := c.cfg.Clients[r.spec.ID]
	switch {
	case !binding.Enabled:
		return []localCheck{skippedCheck(name, label, r.spec.Label+" is disabled, so AIGW keeps no client files for it")}
	case selectErr != nil:
		return []localCheck{skippedCheck(name, label, "No model resolves, so no client files can match it")}
	case binding.Executable == "":
		if discovered, err := c.deps.Discover(r.spec.ID); err == nil && discovered.Executable(r.spec.ID) != "" {
			return []localCheck{failedCheck(name, label, r.spec.Label+" is installed, but AIGW has not written its files yet", "aigw sync", domainreadiness.Deferred)}
		}
		// Activation names the same step for every readiness command.
		install := clientactivation.ProjectionPrerequisites(c.cfg)[r.spec.ID]
		return []localCheck{failedCheck(name, label, r.spec.Label+" is not installed", install, domainreadiness.Deferred)}
	}
	status := c.deps.InspectClient(ctx, c.cfg, r.spec.ID, selected)
	check := passedCheck(name, label, "Match the selection")
	if status.NativeModelOverride {
		r.native = true
		check.Detail = "Match the selection; " + c.admission().ProductName(r.spec.ID) + " keeps its own model preference"
	}
	if !status.Ready {
		check = failedCheck(name, label, status.Issue, status.RepairAction, domainreadiness.Invalid)
	}
	checks := []localCheck{check}
	home := invocation.Home(c.deps.Dependencies)
	for index, observation := range status.Checks {
		target, outside := terminal.HomeTarget(observation.Target, home)
		file := localCheck{Name: observation.ID, State: checkPassed, Detail: "Matches the selection", Target: target, label: targetLabel(r.spec, target, outside, index)}
		if !observation.Ready {
			file.State, file.Detail, file.fix, file.failed = checkFailed, observation.Detail, observation.RepairAction, domainreadiness.Invalid
		}
		checks = append(checks, file)
	}
	return checks
}

// targetLabel names a client file by its path relative to the home directory,
// or, outside it, by its position, saying where it lies.
func targetLabel(spec domain.ClientSpec, target string, outside bool, index int) string {
	position := strconv.Itoa(index + 1)
	switch {
	case target != "":
		return spec.Label + " configuration " + target
	case outside:
		return spec.Label + " configuration target " + position + " (outside the home directory)"
	}
	return spec.Label + " configuration target " + position
}

// noClientProblem explains a check that names no client while none is
// enabled: what activation is waiting for, and its next step.
func noClientProblem(cfg domain.Config, activation clientactivation.Activation) (title, evidence string) {
	switch {
	case len(cfg.Providers) == 0:
		return "AIGW is not set up", "No provider has been added."
	case activation.State == domainreadiness.Unavailable:
		return "No client is enabled", "The credential backend cannot tell which provider has a Token."
	case len(cfg.Models) == 0:
		return "No client is enabled", "No model has been added, so no client can use one."
	}
	for _, id := range cfg.Admission().IDs() {
		if cfg.SelectedModel(id) != "" {
			return "No client is enabled", "Each client that selects a model is disabled."
		}
	}
	return "No client is enabled", "Models have been added, but no client uses one."
}
