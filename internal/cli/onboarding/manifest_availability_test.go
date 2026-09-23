package onboarding

import (
	"net/http"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
)

func TestManifestSetupAvailabilityFollowsTheAdmittedClientRegistry(t *testing.T) {
	executables := map[string]string{}
	for _, clientID := range configuration.AdmittedClientIDs() {
		executables[clientID] = "/installed/" + clientID
	}
	available := manifestSetupAvailableClients(executables)
	for _, clientID := range configuration.AdmittedClientIDs() {
		if !available[clientID] {
			t.Fatalf("admitted installed client %q is unavailable: %#v", clientID, available)
		}
	}
}

func TestManifestCredentialVerificationUsesResolvedProtocolOnce(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	for _, clientID := range []string{configuration.ClientClaudeDesktop, configuration.ClientHermes} {
		cfg.Routes[clientID] = configuration.Route{
			Label: clientID, Account: "team", Model: "claude-test",
			Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
		}
		cfg.SetSelectedRoute(clientID, clientID)
		binding := cfg.Clients[clientID]
		binding.Protocol = configuration.ProtocolAnthropic
		binding.Enabled = true
		cfg.Clients[clientID] = binding
	}
	calls := 0
	runtime := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.String() != "https://team.test/v1/models" || request.Header.Get("X-Api-Key") != "token" {
			t.Fatalf("credential probe = %s, headers %#v", request.URL, request.Header)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: request}, nil
	})}
	connected := map[string]setupCredential{"team": {account: "team", token: "token"}}
	clients := manifestSetupSelectedClients(cfg, connected, map[string]bool{
		configuration.ClientClaudeDesktop: true,
		configuration.ClientHermes:        true,
	})
	if err := verifyManifestSetupCredential(t.Context(), runtime, cfg, "team", "token", clients...); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("equivalent credential probes = %d, want 1", calls)
	}
}
