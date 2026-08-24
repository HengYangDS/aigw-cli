package readiness

import (
	"fmt"
	"strings"

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
	accountName, account := renderActiveService(r, cfg, result)
	attention, selectionCommand, authenticationCommand := renderClientStatus(r, result)
	renderTransportStatus(r, result)
	renderDiagnosticStatus(runtime, r, accountName, account)
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

func renderActiveService(r *presentation.Renderer, cfg configuration.Config, result statusOutput) (string, configuration.Account) {
	r.Section("Active service")
	current := cfg.Profiles[result.Default]
	accountName := current.Account
	account := cfg.Accounts[accountName]
	r.Row("Current profile", current.Label)
	r.Row("Configuration", result.Default)
	if purpose := strings.TrimSpace(current.Purpose); purpose != "" {
		r.Row("Purpose", purpose)
	}
	r.Row("Account", accountName)
	for _, spec := range configuration.AdmittedClientSpecs() {
		if model := current.ModelFor(spec.ID); model != "" {
			r.Row(spec.Label+" model", model)
		}
	}
	r.Row("Model profiles", fmt.Sprintf("%d", result.Profiles))
	return accountName, account
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
				command := "aigw use " + route.SuggestedProfile + " --for " + client
				message += " · " + command
				if selectionCommand == "" {
					selectionCommand = command
				}
			}
			r.Status(presentation.Warn, invocation.Title(client), message)
			attention = true
			continue
		}
		mode := "Explicit override"
		if route.Inherited {
			mode = "Inherits default"
		}
		readiness := route.Profile + " · " + mode + " · Ready"
		state := presentation.OK
		if !route.SecretAvailable || !route.EndpointReady || !route.AdapterReady {
			readiness = route.Profile + " · " + mode + " · Action required"
			if route.AdapterIssue != "" {
				readiness = route.Profile + " · " + mode + " · " + route.AdapterIssue
			}
			state = presentation.Warn
			attention = true
		} else if route.NativeAuthentication == "not_proven" {
			readiness = route.Profile + " · " + mode + " · Projection ready · Native authentication not proven"
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

func renderDiagnosticStatus(runtime invocation.Context, r *presentation.Renderer, accountName string, account configuration.Account) {
	r.Section("Optional diagnostics")
	switch {
	case account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind) && runtime.Accounts.Has(accountName):
		r.Status(presentation.OK, "Precise balance", "Enabled")
	case account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind):
		r.Status(presentation.Warn, "Precise balance", "Disabled")
		r.Detail("aigw account connect " + accountName)
	case account.AccountProbe != nil:
		r.Status(presentation.Info, "Precise balance", "This version does not provide diagnostics for this provider")
	default:
		r.Status(presentation.Info, "Precise balance", "Provider does not expose a probe")
	}
}
