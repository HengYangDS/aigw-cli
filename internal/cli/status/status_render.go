package status

import (
	"strings"

	"aigw-cli/internal/app"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/balanceprobe"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

func renderStatus(runtime invocation.Context, cfg domain.Config, result statusReport) {
	r := invocation.Renderer(runtime)
	// Providers without models are a configuration whose clients select
	// nothing, which the client rows show.
	if len(cfg.Providers) == 0 {
		r.ProductTitle("Not configured")
		r.Section("Get started")
		r.Text("Run the guided setup once to add a provider, its Token and a first model.")
		r.Next("aigw setup")
		return
	}
	r.ProductTitle("Configuration status")
	r.Text("Each client's model and readiness, and the smallest next action.")
	clientIDs := runtime.Synchronizer().ClientIDs()
	admission := runtime.Clients.Admission()
	renderClientStatus(r, admission, result, clientIDs)
	if result.State == domainreadiness.Deferred {
		r.Section("Activation")
		message := "No client is enabled"
		if result.EnabledClients != 0 {
			message = "AIGW has not written the enabled clients' files yet"
		}
		r.Rows(terminal.Field{
			State: terminal.Info, Label: "Client activation", Value: message,
			Details: []string{"Models are added, but no client's endpoint or model has been checked"},
		})
	}
	renderTransportStatus(r, admission, result, clientIDs)
	renderDiagnosticStatus(runtime, r, cfg)
	r.Next(result.NextAction)
}

func renderClientStatus(r *terminal.Renderer, admission domain.Admission, result statusReport, clientIDs []string) {
	r.Section("Clients")
	rows := make([]terminal.Field, 0, len(clientIDs))
	for _, client := range clientIDs {
		spec, _ := admission.Spec(client)
		clientStatus := result.Clients[client]
		message := clientStatus.Model + " · " + clientStatus.State.Label()
		state := terminal.Info
		switch clientStatus.State {
		case domainreadiness.Configured:
			state = terminal.Info
		case domainreadiness.Deferred:
			if clientStatus.Model == "" {
				message = "No model selected"
			}
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			state = terminal.Warn
		}
		if clientStatus.Detail != "" && clientStatus.Model != "" {
			message = clientStatus.Model + " · " + clientStatus.State.Label() + " · " + clientStatus.Detail
		}
		if len(clientStatus.QualifiedModes) > 0 && len(clientStatus.QualifiedPlatforms) > 0 {
			message += " · Qualified: " + strings.Join(clientStatus.QualifiedModes, ", ") + " · " + strings.Join(clientStatus.QualifiedPlatforms, ", ")
		}
		rows = append(rows, terminal.Field{State: state, Label: spec.Label, Value: message})
	}
	r.Rows(rows...)
}

func renderTransportStatus(r *terminal.Renderer, admission domain.Admission, result statusReport, clientIDs []string) {
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
	providerIDs := cfg.SelectedProviderIDs()
	if len(providerIDs) == 0 {
		r.Rows(terminal.Field{State: terminal.Info, Label: "Precise balance", Value: "No client selects a provider"})
		return
	}
	rows := make([]terminal.Field, 0, len(providerIDs))
	for _, id := range providerIDs {
		provider := cfg.Providers[id]
		if provider.BalanceProbe == nil {
			rows = append(rows, terminal.Field{State: terminal.Info, Label: id, Value: "Provider does not expose a balance probe"})
			continue
		}
		if !balanceprobe.Supports(provider.BalanceProbe.Kind) {
			rows = append(rows, terminal.Field{State: terminal.Info, Label: id, Value: "Provider diagnostics unavailable in this version"})
			continue
		}
		available, err := runtime.Providers.Exists(id)
		switch {
		case err != nil:
			// provider show reports the credential backend that could not tell.
			rows = append(rows, terminal.Field{State: terminal.Warn, Label: id, Value: "Balance credentials unknown · aigw provider show " + id})
		case available:
			rows = append(rows, terminal.Field{State: terminal.Info, Label: id, Value: "Balance credentials stored · not verified"})
		default:
			action := "aigw provider edit " + id + " --balance-token-stdin --balance-user-id <user-id>"
			if secret.IsReadOnly(runtime.Secrets) {
				action = "set `" + secret.DiagnosticSystemTokenEnvironmentKey(id) + "` and `" + secret.DiagnosticUserIDEnvironmentKey(id) + "`"
			}
			rows = append(rows, terminal.Field{State: terminal.Warn, Label: id, Value: "Precise balance not ready · " + action})
		}
	}
	r.Rows(rows...)
}
