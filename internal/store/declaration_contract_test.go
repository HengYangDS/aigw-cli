package store

import (
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

func TestValidateKeepsUnselectedBindingsWithinTheDeclarationContract(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*domain.Config)
		want string
	}{
		{"unselected client", func(c *domain.Config) { c.Clients[ClientCodex] = domain.ClientBinding{} }, ""},
		{"relative reader without route", func(c *domain.Config) { c.Clients[ClientCodex] = domain.ClientBinding{CredentialCommand: "reader"} }, "one absolute executable path"},
		{"empty recommendation", func(c *domain.Config) { c.Recommendations[ClientCodex] = domain.ClientRecommendation{} }, "has no primary model"},
		{"invalid family", func(c *domain.Config) { c.Families["bad model"] = domain.Family{Label: "Invalid"} }, "invalid model name"},
		{"blank model ID", func(c *domain.Config) {
			c.Models[domain.NewModelRef("gateway", " ")] = domain.Model{Provider: "gateway", ID: " "}
		}, "empty ID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := declarationConfig()
			cfg.Normalize()
			test.edit(&cfg)
			before := cfg.Clone()
			err := cfg.Validate()
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("declaration error = %v; want %q", err, test.want)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("declaration validation changed the input")
			}
		})
	}
}

func declarationConfig() domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{
		Label: "Gateway",
		Endpoints: domain.Endpoints{
			Responses: "https://gateway.test/openai/v1",
			Anthropic: "https://gateway.test/anthropic",
		},
	}
	cfg.Models[domain.NewModelRef("gateway", "gpt-5.6-sol")] = domain.Model{
		Label:    "Default",
		Provider: "gateway",
		Family:   "gpt-5.6-sol", ID: "gpt-5.6-sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolResponses: {},
			domain.ProtocolAnthropic: {},
		},
	}
	cfg.SetSelectedModel(ClientCodex, domain.NewModelRef("gateway", "gpt-5.6-sol"), "")
	return cfg
}
