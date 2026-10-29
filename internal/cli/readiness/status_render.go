package readiness

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
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
	admission := runtime.Clients.Admission()
	renderClientStatus(r, admission, result, clientIDs)
	if result.State == domainreadiness.Deferred {
		r.Section("Activation")
		message := "No client is enabled"
		if result.EnabledClients != 0 {
			message = "Selected client projection is deferred"
		}
		r.Rows(presentation.Field{
			State: presentation.Info, Label: "Client activation", Value: message,
			Details: []string{"The catalogue is available, but no client endpoint or model has been checked"},
		})
	}
	renderTransportStatus(r, admission, result, clientIDs)
	renderDiagnosticStatus(runtime, r, cfg)
	r.Next(result.NextAction)
}

func renderClientStatus(r *presentation.Renderer, admission configuration.Admission, result statusOutput, clientIDs []string) {
	r.Section("Clients")
	rows := make([]presentation.Field, 0, len(clientIDs))
	for _, client := range clientIDs {
		spec, _ := admission.Spec(client)
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
				message = "No " + spec.Label + " route selected"
			}
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			state = presentation.Warn
		}
		if clientStatus.Detail != "" && clientStatus.Route != "" {
			message = clientStatus.Route + " · " + clientStatus.State.Label() + " · " + clientStatus.Detail
		}
		if len(clientStatus.QualifiedModes) > 0 && len(clientStatus.QualifiedPlatforms) > 0 {
			message += " · Qualified: " + strings.Join(clientStatus.QualifiedModes, ", ") + " · " + strings.Join(clientStatus.QualifiedPlatforms, ", ")
		}
		rows = append(rows, presentation.Field{State: state, Label: spec.Label, Value: message})
	}
	r.Rows(rows...)
}

func renderTransportStatus(r *presentation.Renderer, admission configuration.Admission, result statusOutput, clientIDs []string) {
	rows := []presentation.Field{}
	for _, client := range clientIDs {
		if result.Clients[client].Transport == endpointTransportExternalLoopback {
			rows = append(rows, presentation.Field{State: presentation.Info, Label: admission.Label(client), Value: "Loopback endpoint"})
		}
	}
	if len(rows) == 0 {
		return
	}
	r.Section("Transport")
	r.Rows(rows...)
	r.Detail("Endpoint runtime identity and availability are not inferred from the address")
	r.Detail("AIGW does not start, stop, or configure the endpoint runtime")
}

func renderDiagnosticStatus(runtime invocation.Context, r *presentation.Renderer, cfg configuration.Config) {
	r.Section("Optional diagnostics")
	accountIDs := cfg.SelectedAccountIDs()
	if len(accountIDs) == 0 {
		r.Rows(presentation.Field{State: presentation.Info, Label: "Precise balance", Value: "No selected account"})
		return
	}
	rows := make([]presentation.Field, 0, len(accountIDs))
	for _, accountName := range accountIDs {
		account := cfg.Accounts[accountName]
		if account.AccountProbe == nil {
			rows = append(rows, presentation.Field{State: presentation.Info, Label: accountName, Value: "Provider does not expose a balance probe"})
			continue
		}
		if !providers.Supports(account.AccountProbe.Kind) {
			rows = append(rows, presentation.Field{State: presentation.Info, Label: accountName, Value: "Provider diagnostics unavailable in this version"})
			continue
		}
		available, err := runtime.Accounts.Exists(accountName)
		switch {
		case err != nil:
			rows = append(rows, presentation.Field{State: presentation.Warn, Label: accountName, Value: "Credential metadata unavailable · aigw doctor"})
		case available:
			rows = append(rows, presentation.Field{State: presentation.Info, Label: accountName, Value: "Diagnostic credential present · content not verified"})
		default:
			action := "aigw account diagnostics enable " + accountName
			if secrets.IsReadOnly(runtime.Secrets) {
				action = "set `" + secrets.DiagnosticSystemTokenEnvironmentKey(accountName) + "` and `" + secrets.DiagnosticUserIDEnvironmentKey(accountName) + "`"
			}
			rows = append(rows, presentation.Field{State: presentation.Warn, Label: accountName, Value: "Precise balance not ready · " + action})
		}
	}
	r.Rows(rows...)
}
