package readiness

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
	domainreadiness "aigw-cli/internal/readiness"
	"strings"
)

func renderStatus(runtime invocation.Context, cfg configuration.Config, result statusOutput) {
	r := invocation.Renderer(runtime)
	if len(cfg.Routes) == 0 {
		r.ProductTitle("Not configured")
		r.Section("Get started")
		r.Text("Run the guided setup once to add an Account, Token, and first Route.")
		r.Next("aigw setup")
		return
	}
	r.ProductTitle("Configuration status")
	r.Text("The selected Routes, client readiness, and the smallest next action.")
	clientIDs := invocation.Synchronizer(runtime).ClientIDs()
	attention, nextAction := renderClientStatus(r, result, clientIDs)
	if result.State == domainreadiness.Deferred {
		r.Section("Activation")
		r.Status(presentation.Info, "Client activation", "No client is enabled")
		r.Detail("The catalogue is available, but no client endpoint or model has been checked")
	}
	renderTransportStatus(r, result, clientIDs)
	renderDiagnosticStatus(runtime, r, cfg)
	switch {
	case attention && nextAction != "":
		r.Next(nextAction)
	case result.State == domainreadiness.Deferred && strings.HasPrefix(result.NextAction, "set environment variable "):
		r.Next(result.NextAction)
	case nextAction != "":
		r.Next(nextAction)
	case result.State == domainreadiness.Deferred:
		r.Next(result.NextAction)
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
		clientStatus := result.Clients[client]
		message := clientStatus.Route + " · " + clientStatus.State.Label()
		state := presentation.Info
		switch clientStatus.State {
		case domainreadiness.EndpointChecked, domainreadiness.InferenceChecked:
			state = presentation.OK
		case domainreadiness.Configured:
			state = presentation.Info
		case domainreadiness.Deferred:
			if clientStatus.Route == "" {
				message = "No " + invocation.Title(client) + " route selected"
			}
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			state = presentation.Warn
			attention = true
		}
		if clientStatus.Detail != "" && clientStatus.Route != "" {
			message = clientStatus.Route + " · " + clientStatus.State.Label() + " · " + clientStatus.Detail
		}
		if len(clientStatus.QualifiedModes) > 0 && len(clientStatus.QualifiedPlatforms) > 0 {
			message += " · Qualified: " + strings.Join(clientStatus.QualifiedModes, ", ") + " · " + strings.Join(clientStatus.QualifiedPlatforms, ", ")
		}

		if nextAction == "" && clientStatus.NextAction != "" {
			nextAction = clientStatus.NextAction
		}
		r.Status(state, invocation.Title(client), message)
	}
	return attention, nextAction
}

func renderTransportStatus(r *presentation.Renderer, result statusOutput, clientIDs []string) {
	shown := false
	for _, client := range clientIDs {
		if result.Clients[client].Transport != endpointTransportExternalLoopback {
			continue
		}
		if !shown {
			r.Section("Transport")
			shown = true
		}
		r.Status(presentation.Info, invocation.Title(client), "Loopback endpoint")
	}
	if shown {
		r.Detail("Endpoint runtime identity and availability are not inferred from the address")
		r.Detail("AIGW does not start, stop, or configure the endpoint runtime")
	}
}

func renderDiagnosticStatus(runtime invocation.Context, r *presentation.Renderer, cfg configuration.Config) {
	r.Section("Optional diagnostics")
	accountIDs := cfg.SelectedAccountIDs()
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
			r.Status(presentation.Warn, accountName, "Precise balance disabled · aigw account diagnostics enable "+accountName)
		}
	}
}
