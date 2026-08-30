package readiness

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
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
	attention, selectionCommand, authenticationCommand := renderClientStatus(r, result)
	renderTransportStatus(r, result)
	renderDiagnosticStatus(runtime, r, cfg)
	switch {
	case selectionCommand != "":
		r.Next(selectionCommand)
	case authenticationCommand != "":
		r.Next(authenticationCommand)
	case attention:
		r.Next("aigw repair")
	default:
		r.Next("aigw check")
	}
}

func renderClientStatus(r *presentation.Renderer, result statusOutput) (bool, string, string) {
	r.Section("Clients")
	attention := false
	selectionCommand := ""
	authenticationCommand := ""
	for _, client := range admittedClientIDs() {
		route := result.Routes[client]
		if route.NeedsSelection {
			message := "No " + invocation.Title(client) + " profile selected"
			if route.SuggestedProfile != "" {
				command := "aigw use " + route.SuggestedProfile
				message += " · " + command
				if selectionCommand == "" {
					selectionCommand = command
				}
			}
			r.Status(presentation.Warn, invocation.Title(client), message)
			attention = true
			continue
		}
		readiness := route.Profile + " · Ready"
		state := presentation.OK
		if !route.SecretAvailable || !route.EndpointReady || !route.AdapterReady {
			readiness = route.Profile + " · Action required"
			if route.AdapterIssue != "" {
				readiness = route.Profile + " · " + route.AdapterIssue
			}
			state = presentation.Warn
			attention = true
		} else if route.NativeAuthentication == "not_proven" {
			readiness = route.Profile + " · Projection ready · Native authentication not proven"
			state = presentation.Warn
			authenticationCommand = "aigw adapter auth codex"
		}
		r.Status(state, invocation.Title(client), readiness)
	}
	return attention, selectionCommand, authenticationCommand
}

func renderTransportStatus(r *presentation.Renderer, result statusOutput) {
	for _, client := range admittedClientIDs() {
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
		switch {
		case account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind) && runtime.Accounts.Has(accountName):
			r.Status(presentation.OK, accountName, "Precise balance enabled")
		case account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind):
			r.Status(presentation.Warn, accountName, "Precise balance disabled · aigw account connect "+accountName)
		case account.AccountProbe != nil:
			r.Status(presentation.Info, accountName, "Provider diagnostics unavailable in this version")
		default:
			r.Status(presentation.Info, accountName, "Provider does not expose a balance probe")
		}
	}
}
