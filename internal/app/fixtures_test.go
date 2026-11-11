package app

import (
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"net/http"
)

func configuredCatalog() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	cfg.Models[domain.NewModelRef("gateway", "gpt-codex")] = domain.Model{
		Label: "Codex", Provider: "gateway", Family: "gpt-codex", ID: "gpt-codex",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}},
	}
	cfg.Models[domain.NewModelRef("gateway", "gpt-claude")] = domain.Model{
		Label: "Claude", Provider: "gateway", Family: "gpt-claude", ID: "gpt-claude",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-codex"), "")
	return cfg
}

type catalogHTTPClient func(*http.Request) (*http.Response, error)

func (client catalogHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}
