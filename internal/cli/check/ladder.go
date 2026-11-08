package check

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"aigw-cli/internal/app/diagnostics"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// checker climbs the ladder for each checked client with one invocation's
// capabilities and the configuration that it loaded.
type checker struct {
	deps   invocation.Context
	cfg    domain.Config
	plan   plan
	shared []localCheck
	named  named
}

// plan is what the arguments ask a check for: the depth, and the clients to
// check in admission order, named or else the enabled ones.
type plan struct {
	requested depth
	clients   []string
	named     bool
}

func (c *checker) admission() domain.Admission { return c.cfg.Admission() }

// run evaluates every client of the plan. A client stops at its first failed
// depth, and the others still climb.
func (c *checker) run(ctx context.Context) outcome {
	result := outcome{requested: c.plan.requested, named: c.plan.named, shared: c.shared}
	for _, id := range c.plan.clients {
		result.clients = append(result.clients, c.climb(ctx, id))
	}
	return result
}

// climb runs every depth up to the requested one for one client.
func (c *checker) climb(ctx context.Context, id string) result {
	r := c.local(ctx, id)
	if r.failure == nil && c.plan.requested.runs(depthEndpoint) {
		c.remote(ctx, &r)
	}
	if r.failure == nil && c.plan.requested.runs(depthClient) {
		c.session(ctx, &r)
	}
	return r
}

// remote runs the endpoint depth, and the model depth when the check asks for
// it, with one read of the provider's Token for both. A client that
// authenticates itself, or whose Token an external helper supplies, cannot run
// either: only a session of it proves its authentication.
func (c *checker) remote(ctx context.Context, r *result) {
	product := c.admission().ProductName(r.spec.ID)
	if !r.runtime.UsesAIGWCredentialStore() {
		reason := product + " uses its own authentication, which only a session of it proves"
		if r.runtime.RequiresProviderToken() {
			reason = "An external credential helper gives " + product + " its Token, which only a session of it proves"
		}
		r.skipDepth(depthEndpoint, reason)
		if c.plan.requested.runs(depthModel) {
			r.skipDepth(depthModel, reason)
		}
		r.deeper = c.sessionCheck(*r)
		return
	}
	token := c.named.token
	if token == "" {
		var err error
		if token, err = c.deps.Secrets.Get(r.runtime.ProviderID); err != nil {
			r.fail(c.tokenReadFailure(r, err))
			return
		}
	}
	endpoint := diagnostics.ProbeBounded(ctx, c.deps.HTTP, r.runtime, token, diagnostics.ScopeEndpoint)
	switch {
	case endpoint.Kind == diagnostics.Healthy:
		r.reached = depthEndpoint
	case endpoint.Kind == diagnostics.EndpointMismatch && endpoint.HTTPStatus == http.StatusNotFound && r.spec.ModelDiscoveryOptional:
		// The model depth proves the Token on such an endpoint.
		r.skipDepth(depthEndpoint, "The endpoint lists no models, so a request without a model cannot prove the Token")
		r.deeper = c.deeperCheck(r.spec.ID, depthModel)
	default:
		r.fail(c.probeFailure(r, depthEndpoint, endpoint))
		return
	}
	if !c.plan.requested.runs(depthModel) {
		return
	}
	r.deeper = ""
	if r.native {
		r.skipDepth(depthModel, product+" picks its own model, so AIGW does not know which model it sends")
		r.deeper = c.sessionCheck(*r)
		return
	}
	inference := diagnostics.ProbeBounded(ctx, c.deps.HTTP, r.runtime, token, diagnostics.ScopeInference)
	if inference.Kind != diagnostics.Healthy {
		r.fail(c.probeFailure(r, depthModel, inference))
		return
	}
	r.reached = depthModel
}

// session runs a real session of the client, which proves the client depth.
// Claude Desktop needs acceptance inside the app, and a disabled client has no
// client files to run a session with.
func (c *checker) session(ctx context.Context, r *result) {
	switch {
	case r.spec.InAppAcceptance:
		r.skipDepth(depthClient, r.spec.Label+" needs acceptance inside the app")
		return
	case !r.enabled:
		r.skipDepth(depthClient, r.spec.Label+" is disabled, so AIGW keeps no client files for a session")
		return
	}
	// A named model takes its own isolated session, which its retry names.
	verification, err := c.deps.Synchronizer().Verify(ctx, c.cfg, r.spec.ID, r.runtime, c.named.name())
	if err != nil {
		problem := terminal.ProblemFor(err)
		r.fail(failure{depth: depthClient, state: domainreadiness.Degraded, problem: problem.Title, fix: problem.Fix, cause: err})
		return
	}
	r.reached, r.session, r.deeper = depthClient, &verification, ""
}

// sessionCheck is the check that runs a session of the client, which proves
// what a skipped depth could not, or nothing when the client cannot run one.
func (c *checker) sessionCheck(r result) string {
	if r.spec.InAppAcceptance || !r.enabled {
		return ""
	}
	return c.deeperCheck(r.spec.ID, depthClient)
}

// deeperCheck is the check of one client to d, which proves what a skipped
// depth could not, or nothing when this check already goes that deep.
func (c *checker) deeperCheck(id string, d depth) string {
	if c.plan.requested.runs(d) {
		return ""
	}
	return c.checkCommand(id, d)
}

// checkCommand is the check of one client to d, which keeps the model that
// --model named, and the one-time Token at the depths that send it.
func (c *checker) checkCommand(id string, d depth) string {
	return checkCommand(id, c.named, d)
}

// checkCommand is the check of client to d, which keeps the model that
// --model named, and its one-time Token at the depths that send it, or prose
// for a model name that would need quoting.
func checkCommand(client string, model named, d depth) string {
	words := []string{"check", client}
	if name := model.name(); name != "" {
		words = append(words, name)
	}
	command := invocation.Command(words...)
	if command == "" {
		return "Run the same check again with --depth " + string(d) + "."
	}
	if name := model.name(); name != "" {
		command = strings.TrimSuffix(command, " "+name) + " --model " + name
		if model.token != "" && (d == depthEndpoint || d == depthModel) {
			command += " --token-stdin"
		}
	}
	if d != depthModel {
		command += " --depth " + string(d)
	}
	return command
}

// tokenReadFailure is the failure of a Token read before the endpoint depth:
// a Token removed since its metadata was read, or a backend that failed.
func (c *checker) tokenReadFailure(r *result, err error) failure {
	provider := r.runtime.ProviderID
	if errors.Is(err, secret.ErrNotFound) {
		return failure{depth: depthEndpoint, state: domainreadiness.Deferred, problem: "Provider " + provider + " has no Token", fix: c.rotateFix(provider, false), cause: err}
	}
	return failure{depth: depthEndpoint, state: domainreadiness.Unavailable, problem: "The credential backend could not return provider " + provider + "'s Token", fix: backendFix, cause: err}
}

// rotateFix replaces provider's Token: `aigw provider rotate`, or, from the
// read-only environment backend, the variable that holds it. rejected marks a
// Token that the provider refused, whose variable needs another value.
func (c *checker) rotateFix(provider string, rejected bool) string {
	if !secret.IsReadOnly(c.deps.Secrets) {
		return "aigw provider rotate " + provider
	}
	if rejected {
		return "Set " + secret.EnvironmentKey(provider) + " to a Token that the provider accepts."
	}
	return "Set " + secret.EnvironmentKey(provider) + " to the provider's Token."
}

// probeFailure is the failure of an authenticated request at d: the HTTP
// status and what it means, the state that it leaves the client in, and the
// fix that acts on its cause. Only an unreachable endpoint, a timeout (HTTP
// 408), a rate limit (429) and a provider failure (5xx) name the same check
// again.
func (c *checker) probeFailure(r *result, d depth, observed diagnostics.Result) failure {
	f := failure{depth: d, state: domainreadiness.ProbeState(observed.Kind), probe: &observed, problem: sentence(observed.Summary)}
	if observed.HTTPStatus != 0 {
		f.problem = fmt.Sprintf("HTTP %d: %s", observed.HTTPStatus, observed.Summary)
	}
	provider := r.runtime.ProviderID
	switch observed.Kind {
	case diagnostics.InvalidToken:
		f.fix = c.rotateFix(provider, true)
	case diagnostics.TokenDisabled:
		f.fix = "Enable the Token in provider " + provider + "'s console."
	case diagnostics.TokenRestricted:
		f.fix = "Allow the Token to use " + r.model + " in provider " + provider + "'s console."
	case diagnostics.QuotaExhausted:
		f.fix = "Add quota to the Token in provider " + provider + "'s console."
	case diagnostics.EndpointMismatch:
		f.fix = "aigw provider edit " + provider + " --endpoint " + r.runtime.Protocol.Name() + "=<url>"
	case diagnostics.ModelUnresolved:
		f.fix = "aigw model show " + r.model
	case diagnostics.ModelUnavailable:
		f.fix = "aigw use " + r.spec.ID + " <provider/model>"
		if observed.HTTPStatus >= http.StatusInternalServerError {
			f.fix = c.checkCommand(r.spec.ID, c.plan.requested)
		}
	case diagnostics.Timeout, diagnostics.RateLimited, diagnostics.UpstreamFailure, diagnostics.NetworkFailure:
		f.fix = c.checkCommand(r.spec.ID, c.plan.requested)
	case diagnostics.Healthy, diagnostics.Unexpected:
		f.fix = "aigw provider show " + provider
	default:
		f.fix = "aigw provider show " + provider
	}
	return f
}

// sentence makes a clause the start of a sentence.
func sentence(clause string) string {
	if clause == "" {
		return ""
	}
	return strings.ToUpper(clause[:1]) + clause[1:]
}
