package status

import (
	"aigw-cli/internal/app"
	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// statusReport is `aigw status`'s document: the credential backend, the local
// readiness of every admitted client, the number of models added, and the
// smallest next action.
type statusReport struct {
	CredentialBackend secret.BackendSelection `json:"credential_backend"`
	Clients           map[string]clientState  `json:"clients"`
	Models            int                     `json:"models"`
	EnabledClients    int                     `json:"enabled_clients"`
	State             domainreadiness.State   `json:"state,omitempty"`
	NextAction        string                  `json:"next_action,omitempty"`
	NextDetail        string                  `json:"next_detail,omitempty"`
	// observed is each client's readiness as the readiness owner reports it,
	// with its model's configuration key.
	observed map[string]app.ClientStatus
}

// clientState is one client's local readiness as status reports it: its model
// as provider/model and the provider that serves it, its state with the
// detail and next step, and the local facts behind them.
type clientState struct {
	State      domainreadiness.State `json:"state"`
	Model      string                `json:"model,omitempty"`
	Provider   string                `json:"provider,omitempty"`
	Detail     string                `json:"detail,omitempty"`
	NextAction string                `json:"next_action,omitempty"`
	NextDetail string                `json:"next_detail,omitempty"`
	// FilesDeferred marks an enabled client whose files AIGW has not written,
	// because the client was not installed when it was enabled, and
	// NativeModel a Claude Code that keeps its own model preference.
	FilesDeferred      bool                      `json:"client_files_deferred,omitempty"`
	NativeModel        bool                      `json:"native_model_preference,omitempty"`
	Authentication     domain.Authentication     `json:"authentication"`
	EndpointConfigured bool                      `json:"endpoint_configured"`
	Transport          app.EndpointTransportKind `json:"transport,omitempty"`
	FilesReady         bool                      `json:"client_files_ready"`
	QualifiedModes     []string                  `json:"qualified_modes,omitempty"`
	QualifiedPlatforms []string                  `json:"qualified_platforms,omitempty"`
}

// clientStates reports each observed client with its model named
// provider/model.
func clientStates(cfg domain.Config, observed map[string]app.ClientStatus) map[string]clientState {
	states := make(map[string]clientState, len(observed))
	for id, status := range observed {
		states[id] = clientState{
			State: status.State, Model: status.Model.String(), Provider: status.Provider,
			Detail: status.Detail, NextAction: status.NextAction, NextDetail: status.NextDetail,
			FilesDeferred: status.ProjectionDeferred, NativeModel: status.NativeModelOverride,
			Authentication: status.Authentication, EndpointConfigured: status.EndpointConfigured, Transport: status.Transport,
			FilesReady: status.ProjectionReady, QualifiedModes: status.QualifiedModes, QualifiedPlatforms: status.QualifiedPlatforms,
		}
	}
	return states
}

// modelCount counts the models added, as `aigw model list` lists them.
func modelCount(cfg domain.Config) int { return len(cfg.Models) }

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
		Clients:           clientStates(cfg, clients),
		Models:            modelCount(cfg),
		EnabledClients:    activation.EnabledClients,
		State:             activation.State,
		NextAction:        activation.NextActionFor(orderedClientStates(runtime, clients)),
		observed:          clients,
	}
}
