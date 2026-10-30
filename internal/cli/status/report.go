package status

import (
	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/app"
	"aigw-cli/internal/domain"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secret"
)

// statusReport is the local readiness of every admitted client.
type statusReport struct {
	CredentialBackend secret.BackendSelection     `json:"credential_backend"`
	Clients           map[string]app.ClientStatus `json:"clients"`
	Routes            int                         `json:"routes"`
	EnabledClients    int                         `json:"enabled_clients"`
	State             domainreadiness.State       `json:"state,omitempty"`
	NextAction        string                      `json:"next_action,omitempty"`
}

func orderedClientStates(runtime app.Dependencies, clients map[string]app.ClientStatus) []domainreadiness.Client {
	ordered := make([]domainreadiness.Client, 0, len(clients))
	for _, clientID := range runtime.Synchronizer().ClientIDs() {
		ordered = append(ordered, clients[clientID].Client)
	}
	return ordered
}

// collectStatus observes the credential backend and every admitted client
// without reading Token values or changing state.
func collectStatus(runtime app.Dependencies, cfg domain.Config) statusReport {
	backend, backendErr := secret.Inspect(runtime.Secrets)
	if backendErr != nil {
		backend.RecoveryAction = domainreadiness.CredentialBackendRecovery
	}
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	clients := app.InspectStatusClients(runtime, cfg, &activation, "")
	return statusReport{
		CredentialBackend: backend,
		Clients:           clients,
		Routes:            len(cfg.Routes),
		EnabledClients:    activation.EnabledClients,
		State:             activation.State,
		NextAction:        activation.NextActionFor(orderedClientStates(runtime, clients)),
	}
}
