package readiness

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
	domainreadiness "aigw-cli/internal/readiness"
)

func renderStatus(runtime invocation.Context, cfg configuration.Config, result statusOutput) {
	r := Renderer(runtime)
	if len(cfg.Profiles) == 0 {
		r.ProductTitle("Not configured")
		r.Section("Get started")
		r.Text("Run the guided setup once to add a service, token, and first model profile.")
		r.Next("aigw setup")
		return
	}
	r.ProductTitle("Ready view")
	r.Text("The active service, client readiness, and the smallest next action.")
	clientIDs := invocation.Synchronizer(runtime).ClientIDs()
	attention, nextAction := renderClientStatus(r, result, clientIDs)
	renderTransportStatus(r, result, clientIDs)
	renderDiagnosticStatus(runtime, r, cfg)
	switch {
	case nextAction != "":
		r.Next(nextAction)
	case attention:
		r.Next("aigw repair")
	default:
		r.Next("aigw check")
	}
}

func renderClientStatus(r *presentation.Renderer, result statusOutput, clientIDs []string) (bool, string) {
	r.Section("Clients")
	attention := false
	nextAction := ""
	for _, client := range clientIDs {
		route := result.Routes[client]
		message := route.Profile + " · " + route.State.Label()
		state := presentation.Info
		switch route.State {
		case domainreadiness.Ready:
			state = presentation.OK
		case domainreadiness.Configured:
			state = presentation.Info
		case domainreadiness.Deferred:
			if route.Profile == "" {
				message = "No " + invocation.Title(client) + " profile selected"
			}
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			state = presentation.Warn
			attention = true
		}
		if route.Detail != "" && route.Profile != "" {
			message = route.Profile + " · " + route.State.Label() + " · " + route.Detail
		}
		if route.NativeAuthentication == "not_proven" {
			message = route.Profile + " · Projection ready · Native authentication not proven"
			state = presentation.Warn
			attention = true
			if nextAction == "" {
				nextAction = "aigw adapter auth codex"
			}
		}
		if nextAction == "" && route.NextAction != "" {
			nextAction = route.NextAction
		}
		r.Status(state, invocation.Title(client), message)
	}
	return attention, nextAction
}

func renderTransportStatus(r *presentation.Renderer, result statusOutput, clientIDs []string) {
	for _, client := range clientIDs {
		if result.Routes[client].Transport != "external_loopback" {
			continue
		}
		r.Section("Transport")
		r.Status(presentation.Info, invocation.Title(client), "External loopback compatibility layer")
		r.Detail(invocation.Title(client) + " requests use the external listener")
		r.Detail("AIGW does not start, stop, or configure it")
		return
	}
}

func renderDiagnosticStatus(runtime invocation.Context, r *presentation.Renderer, cfg configuration.Config) {
	r.Section("Optional diagnostics")
	accountIDs := cfg.RoutedAccountIDs()
	if len(accountIDs) == 0 {
		r.Status(presentation.Info, "Precise balance", "No selected account")
		return
	}
	for _, accountName := range accountIDs {
		account := cfg.Accounts[accountName]
		if account.AccountProbe == nil {
			r.Status(presentation.Info, accountName, "Provider does not expose a balance probe")
			continue
		}
		if !providers.Supports(account.AccountProbe.Kind) {
			r.Status(presentation.Info, accountName, "Provider diagnostics unavailable in this version")
			continue
		}
		available, err := runtime.Accounts.Exists(accountName)
		switch {
		case err != nil:
			r.Status(presentation.Warn, accountName, "Credential metadata unavailable · aigw doctor")
		case available:
			r.Status(presentation.OK, accountName, "Precise balance enabled")
		default:
			r.Status(presentation.Warn, accountName, "Precise balance disabled · aigw account connect "+accountName)
		}
	}
}
