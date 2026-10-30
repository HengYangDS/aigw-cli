package status

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/providers"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
	"strings"

	"aigw-cli/internal/app"
)

func renderStatus(runtime invocation.Context, cfg domain.Config, result app.StatusReport) {
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
	clientIDs := runtime.Synchronizer().ClientIDs()
	admission := runtime.Clients.Admission()
	renderClientStatus(r, admission, result, clientIDs)
	if result.State == domainreadiness.Deferred {
		r.Section("Activation")
		message := "No client is enabled"
		if result.EnabledClients != 0 {
			message = "Selected client projection is deferred"
		}
		r.Rows(terminal.Field{
			State: terminal.Info, Label: "Client activation", Value: message,
			Details: []string{"The catalogue is available, but no client endpoint or model has been checked"},
		})
	}
	renderTransportStatus(r, admission, result, clientIDs)
	renderDiagnosticStatus(runtime, r, cfg)
	r.Next(result.NextAction)
}

func renderClientStatus(r *terminal.Renderer, admission domain.Admission, result app.StatusReport, clientIDs []string) {
	r.Section("Clients")
	rows := make([]terminal.Field, 0, len(clientIDs))
	for _, client := range clientIDs {
		spec, _ := admission.Spec(client)
		clientStatus := result.Clients[client]
		message := clientStatus.Route + " · " + clientStatus.State.Label()
		state := terminal.Info
		switch clientStatus.State {
		case domainreadiness.EndpointChecked, domainreadiness.InferenceChecked:
			state = terminal.OK
		case domainreadiness.Configured:
			state = terminal.Info
		case domainreadiness.Deferred:
			if clientStatus.Route == "" {
				message = "No " + spec.Label + " route selected"
			}
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			state = terminal.Warn
		}
		if clientStatus.Detail != "" && clientStatus.Route != "" {
			message = clientStatus.Route + " · " + clientStatus.State.Label() + " · " + clientStatus.Detail
		}
		if len(clientStatus.QualifiedModes) > 0 && len(clientStatus.QualifiedPlatforms) > 0 {
			message += " · Qualified: " + strings.Join(clientStatus.QualifiedModes, ", ") + " · " + strings.Join(clientStatus.QualifiedPlatforms, ", ")
		}
		rows = append(rows, terminal.Field{State: state, Label: spec.Label, Value: message})
	}
	r.Rows(rows...)
}

func renderTransportStatus(r *terminal.Renderer, admission domain.Admission, result app.StatusReport, clientIDs []string) {
	rows := []terminal.Field{}
	for _, client := range clientIDs {
		if result.Clients[client].Transport == app.EndpointTransportExternalLoopback {
			rows = append(rows, terminal.Field{State: terminal.Info, Label: admission.Label(client), Value: "Loopback endpoint"})
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

func renderDiagnosticStatus(runtime invocation.Context, r *terminal.Renderer, cfg domain.Config) {
	r.Section("Optional diagnostics")
	accountIDs := cfg.SelectedAccountIDs()
	if len(accountIDs) == 0 {
		r.Rows(terminal.Field{State: terminal.Info, Label: "Precise balance", Value: "No selected account"})
		return
	}
	rows := make([]terminal.Field, 0, len(accountIDs))
	for _, accountName := range accountIDs {
		account := cfg.Accounts[accountName]
		if account.AccountProbe == nil {
			rows = append(rows, terminal.Field{State: terminal.Info, Label: accountName, Value: "Provider does not expose a balance probe"})
			continue
		}
		if !providers.Supports(account.AccountProbe.Kind) {
			rows = append(rows, terminal.Field{State: terminal.Info, Label: accountName, Value: "Provider diagnostics unavailable in this version"})
			continue
		}
		available, err := runtime.Accounts.Exists(accountName)
		switch {
		case err != nil:
			rows = append(rows, terminal.Field{State: terminal.Warn, Label: accountName, Value: "Credential metadata unavailable · aigw doctor"})
		case available:
			rows = append(rows, terminal.Field{State: terminal.Info, Label: accountName, Value: "Diagnostic credential present · content not verified"})
		default:
			action := "aigw account diagnostics enable " + accountName
			if secret.IsReadOnly(runtime.Secrets) {
				action = "set `" + secret.DiagnosticSystemTokenEnvironmentKey(accountName) + "` and `" + secret.DiagnosticUserIDEnvironmentKey(accountName) + "`"
			}
			rows = append(rows, terminal.Field{State: terminal.Warn, Label: accountName, Value: "Precise balance not ready · " + action})
		}
	}
	r.Rows(rows...)
}
