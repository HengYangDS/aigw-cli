package check

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/app/diagnostics"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/domain"
)

// depth is one rung of the ladder of evidence that `aigw check` climbs for
// each client. A check to one depth runs every depth before it.
type depth string

const (
	// depthLocal proves the configuration, the Token's presence, the client
	// files and the environment, without a request.
	depthLocal depth = "local"
	// depthEndpoint proves that the provider answers one authenticated
	// request that names no model.
	depthEndpoint depth = "endpoint"
	// depthModel proves that the selected model answers one bounded inference
	// request.
	depthModel depth = "model"
	// depthClient proves that a real client session completes.
	depthClient depth = "client"
)

// depths are the rungs in the order in which a check climbs them.
var depths = []depth{depthLocal, depthEndpoint, depthModel, depthClient}

// parseDepth returns the depth that text names: model, the default, for no
// text.
func parseDepth(text string) (depth, error) {
	if text == "" {
		return depthModel, nil
	}
	for _, candidate := range depths {
		if string(candidate) == text {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("--depth must be local, endpoint, model or client, not %q; run `aigw check --help`", text)
}

// runs reports whether a check to d runs other.
func (d depth) runs(other depth) bool {
	return slices.Index(depths, other) <= slices.Index(depths, d)
}

// title is the depth as the first word of a sentence.
func (d depth) title() string { return strings.ToUpper(string(d[:1])) + string(d[1:]) }

// skip is a depth that a client cannot run, with the reason. A skip is not a
// failure.
type skip struct {
	Depth  depth  `json:"depth"`
	Reason string `json:"reason"`
}

// failure is the depth at which a client stopped: what happened, the state
// that it leaves the client in, the one fix, and the request that failed, if
// one did.
type failure struct {
	depth   depth
	state   domainreadiness.State
	problem string
	fix     string
	probe   *diagnostics.Result
	// cause is the error behind the failure, which no output shows.
	cause error
}

// result is one checked client's climb: the model that the check used, the
// deepest depth that it passed, the depths that it skipped, its local checks,
// and its failure, if it stopped at one.
type result struct {
	spec     domain.ClientSpec
	enabled  bool
	runtime  domain.Runtime
	model    string
	resolved bool
	reached  depth
	skipped  []skip
	checks   []localCheck
	failure  *failure
	// native marks a Claude Code that keeps its own model preference, whose
	// model AIGW does not choose.
	native bool
	// session is the identity of the client that a session ran.
	session *adapter.Verification
	// deeper is the check that proves what a depth that this one skipped
	// could not, which only a client session proves.
	deeper string
}

// skipDepth records that the client cannot run d, and why.
func (r *result) skipDepth(d depth, reason string) {
	r.skipped = append(r.skipped, skip{Depth: d, Reason: reason})
}

// fail records that the client stopped at d.
func (r *result) fail(f failure) { r.failure = &f }

// outcome is one check's whole evaluation.
type outcome struct {
	requested depth
	// named marks a check of the clients that its arguments name, which
	// covers them even when they are disabled.
	named   bool
	shared  []localCheck
	clients []result
}

// ok is the check's exit rule: a check that covers no client, which happens
// only when it names none and none is enabled, fails, and so does one in
// which a checked client failed. A skipped depth fails nothing.
func (o outcome) ok() bool {
	if len(o.clients) == 0 {
		return false
	}
	return !slices.ContainsFunc(o.clients, func(client result) bool { return client.failure != nil })
}

// err is the check's failure for its caller, which wraps the cause of each
// client's failure that has one; the output already said what happened.
func (o outcome) err() error {
	errs := []error{errors.New(o.problem())}
	for _, client := range o.clients {
		if client.failure != nil && client.failure.cause != nil {
			errs = append(errs, client.failure.cause)
		}
	}
	return errors.Join(errs...)
}

// enabled counts the checked clients that are enabled.
func (o outcome) enabled() int {
	count := 0
	for _, client := range o.clients {
		if client.enabled {
			count++
		}
	}
	return count
}
