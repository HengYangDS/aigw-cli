package check

import (
	"fmt"

	"aigw-cli/internal/app"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// schemaVersion is the version of `aigw check`'s JSON document, which
// replaces the documents of check, doctor, test and verify, the highest of
// which was 2.
const schemaVersion = 3

// document is `aigw check`'s JSON document: the depth asked for, each checked
// client's climb with its local checks, the checks that every client shares,
// and the next step.
type document struct {
	Depth             depth                   `json:"depth"`
	Clients           []clientDocument        `json:"clients"`
	Checks            []localCheck            `json:"checks"`
	CredentialBackend secret.BackendSelection `json:"credential_backend"`
	EnabledClients    int                     `json:"enabled_clients"`
	State             domainreadiness.State   `json:"state,omitempty"`
	Problem           string                  `json:"problem,omitempty"`
	NextAction        string                  `json:"next_action,omitempty"`
	NextDetail        string                  `json:"next_detail,omitempty"`
}

// clientDocument is one checked client's climb: its model as provider/model,
// the deepest depth that it passed, each depth that it skipped with the
// reason, every local check, and its failure, with one next step.
type clientDocument struct {
	ID                    string                    `json:"id"`
	Enabled               bool                      `json:"enabled"`
	Model                 string                    `json:"model,omitempty"`
	Provider              string                    `json:"provider,omitempty"`
	Protocol              string                    `json:"protocol,omitempty"`
	Authentication        domain.Authentication     `json:"authentication,omitempty"`
	Transport             app.EndpointTransportKind `json:"transport,omitempty"`
	State                 domainreadiness.State     `json:"state"`
	Depth                 depth                     `json:"depth,omitempty"`
	Skipped               []skip                    `json:"skipped"`
	Checks                []localCheck              `json:"checks"`
	Failure               *failureDocument          `json:"failure,omitempty"`
	NativeModelPreference bool                      `json:"native_model_preference,omitempty"`
	Session               *sessionDocument          `json:"session,omitempty"`
	NextAction            string                    `json:"next_action,omitempty"`
	NextDetail            string                    `json:"next_detail,omitempty"`
}

// failureDocument is the depth at which a client stopped, with the
// classification of the request that failed there, if one did.
type failureDocument struct {
	Depth      depth  `json:"depth"`
	Problem    string `json:"problem"`
	Kind       string `json:"kind,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Retryable  bool   `json:"retryable,omitempty"`
	Attempts   int    `json:"attempts,omitempty"`
}

// sessionDocument is the identity of the client that a session ran, measured
// for that session, and what the session exercised.
type sessionDocument struct {
	ClientVersion string `json:"client_version,omitempty"`
	ClientSHA256  string `json:"client_sha256,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

// documented is the check as its document reports it.
func (o outcome) documented(backend secret.BackendSelection) document {
	doc := document{Depth: o.requested, Clients: []clientDocument{}, Checks: documentedChecks(o.shared), CredentialBackend: backend, EnabledClients: o.enabled()}
	for _, client := range o.clients {
		doc.Clients = append(doc.Clients, client.documented())
	}
	doc.Problem = o.problem()
	doc.NextAction, doc.NextDetail = terminal.NextSteps(o.next())
	return doc
}

func (r *result) documented() clientDocument {
	doc := clientDocument{
		ID: r.spec.ID, Enabled: r.enabled, Model: r.model, State: domainreadiness.Configured, Depth: r.reached,
		Skipped: append([]skip{}, r.skipped...), Checks: documentedChecks(r.checks), NativeModelPreference: r.native,
	}
	if r.resolved {
		doc.Provider, doc.Protocol, doc.Authentication = r.runtime.ProviderID, r.runtime.Protocol.Name(), r.runtime.Authentication
		doc.Transport = app.EndpointTransport(r.runtime.Endpoint)
	}
	if r.session != nil {
		doc.Session = &sessionDocument{ClientVersion: r.session.Version, ClientSHA256: r.session.SHA256, Scope: r.session.Scope}
	}
	next := r.deeper
	if r.failure != nil {
		doc.State, next = r.failure.state, r.failure.fix
		doc.Failure = &failureDocument{Depth: r.failure.depth, Problem: r.failure.problem}
		if probe := r.failure.probe; probe != nil {
			doc.Failure.Kind, doc.Failure.HTTPStatus = string(probe.Kind), probe.HTTPStatus
			doc.Failure.Retryable, doc.Failure.Attempts = probe.Retryable, probe.Attempts
		}
	}
	doc.NextAction, doc.NextDetail = terminal.NextSteps(next)
	return doc
}

// documentedChecks splits each failed check's fix into a command that a
// script can run as it stands, or guidance.
func documentedChecks(checks []localCheck) []localCheck {
	documented := make([]localCheck, 0, len(checks))
	for _, check := range checks {
		check.NextAction, check.NextDetail = terminal.NextSteps(check.fix)
		documented = append(documented, check)
	}
	return documented
}

// problem summarizes the clients that failed, or nothing when none did.
func (o outcome) problem() string {
	var failed []result
	for _, client := range o.clients {
		if client.failure != nil {
			failed = append(failed, client)
		}
	}
	switch len(failed) {
	case 0:
		return ""
	case 1:
		return failed[0].spec.Label + " stopped at the " + string(failed[0].failure.depth) + " depth"
	default:
		return fmt.Sprintf("%d of %d checked clients stopped before the %s depth", len(failed), len(o.clients), o.requested)
	}
}

// next is the check's one next step: the first failed client's fix, or else
// the first deeper check that a skipped depth names.
func (o outcome) next() string {
	for _, client := range o.clients {
		if client.failure != nil {
			return client.failure.fix
		}
	}
	for _, client := range o.clients {
		if client.deeper != "" {
			return client.deeper
		}
	}
	return ""
}

// render writes the check for a person: one row per checked client, with the
// depth that it reached, or at which it stopped, and why; every local check
// when verbose asks for them; and the next step.
func (o outcome) render(runtime invocation.Context, verbose bool) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Check")
	r.Section("Clients")
	rows := make([]terminal.Field, 0, len(o.clients))
	for _, client := range o.clients {
		rows = append(rows, client.row())
	}
	r.Rows(rows...)
	if verbose {
		r.Section("Local checks")
		checks := checkRows(o.shared)
		for _, client := range o.clients {
			checks = append(checks, checkRows(client.checks)...)
		}
		r.Rows(checks...)
	}
	if o.ok() {
		r.Section("Result")
		r.Success("No checked client failed")
	}
	r.Next(o.next())
	return r.Err()
}

// row is one client's line: its model and the depth that it reached, or the
// depth at which it stopped, with each skip and its reason, the session's
// client, and the problem and its fix.
func (r *result) row() terminal.Field {
	shown, state := r.reached, terminal.OK
	if r.failure != nil {
		shown, state = r.failure.depth, terminal.Fail
	}
	field := terminal.Field{State: state, Label: r.spec.Label, Value: string(shown)}
	if r.model != "" {
		field.Value = r.model + " · " + string(shown)
	}
	// AIGW points the client at a loopback endpoint; it does not run it.
	loopback := r.resolved && app.EndpointTransport(r.runtime.Endpoint) == app.EndpointTransportExternalLoopback
	if r.failure != nil {
		field.Details = append(field.Details, r.failure.problem, "Fix: "+r.failure.fix)
		if loopback && r.failure.probe != nil {
			field.Details = append(field.Details, r.spec.Label+" uses a loopback endpoint; AIGW does not manage the endpoint runtime")
		}
		return field
	}
	if loopback && r.reached != depthLocal {
		field.Details = append(field.Details, r.spec.Label+" uses a loopback endpoint; AIGW does not manage the endpoint runtime")
	}
	for _, check := range r.checks {
		if check.State == checkSkipped {
			field.Details = append(field.Details, sentence(check.label)+" skipped: "+check.Detail)
		}
	}
	for _, skipped := range r.skipped {
		field.Details = append(field.Details, skipped.Depth.title()+" skipped: "+skipped.Reason)
	}
	if r.session != nil {
		if r.session.Version != "" || r.session.SHA256 != "" {
			field.Details = append(field.Details, fmt.Sprintf("%s %s · SHA-256 %s", r.spec.Label, r.session.Version, r.session.SHA256))
		}
		if r.session.Scope != "" {
			field.Details = append(field.Details, r.session.Scope)
		}
	}
	if r.deeper != "" {
		field.Details = append(field.Details, "Deeper check: "+r.deeper)
	}
	return field
}

// checkRows lists local checks as rows: passed, failed with its fix, or
// skipped with the reason.
func checkRows(checks []localCheck) []terminal.Field {
	rows := make([]terminal.Field, 0, len(checks))
	for _, check := range checks {
		row := terminal.Field{State: terminal.OK, Label: check.label, Value: check.Detail}
		switch check.State {
		case checkFailed:
			row.State, row.Details = terminal.Fail, []string{"Fix: " + check.fix}
		case checkSkipped:
			row.State, row.Value = terminal.Info, "Skipped: "+check.Detail
		}
		rows = append(rows, row)
	}
	return rows
}
