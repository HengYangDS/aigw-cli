package store

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

// document is aigw.toml in schema 8: providers with their models, families,
// recommendations and clients. A team file has the same form without
// clients. Its arrays are the configuration's display order.
type document struct {
	Version         int                               `toml:"version"`
	Providers       []providerDocument                `toml:"providers,omitempty"`
	Families        []familyDocument                  `toml:"families,omitempty"`
	Recommendations map[string]recommendationDocument `toml:"recommendations,omitempty"`
	Clients         map[string]clientDocument         `toml:"clients,omitempty"`
}

type providerDocument struct {
	ID        string            `toml:"id"`
	Label     string            `toml:"label"`
	Balance   *balanceDocument  `toml:"balance,omitempty,inline"`
	Endpoints endpointsDocument `toml:"endpoints,inline"`
	Models    []modelDocument   `toml:"models,omitempty"`
}

type balanceDocument struct {
	Kind    string `toml:"kind"`
	BaseURL string `toml:"base_url"`
}

type endpointsDocument struct {
	Anthropic string `toml:"anthropic,omitempty"`
	Responses string `toml:"responses,omitempty"`
	Chat      string `toml:"chat,omitempty"`
}

type modelDocument struct {
	ID         string                                          `toml:"id"`
	Family     string                                          `toml:"family,omitempty"`
	Label      string                                          `toml:"label,omitempty"`
	Protocols  map[domain.EndpointProtocol][]domain.Capability `toml:"protocols,inline"`
	Deprecated bool                                            `toml:"deprecated,omitempty"`
}

type familyDocument struct {
	ID    string `toml:"id"`
	Label string `toml:"label"`
}

type recommendationDocument struct {
	Primary      selectionDocument   `toml:"primary,inline"`
	Alternatives []selectionDocument `toml:"alternatives,omitempty,inline"`
}

type selectionDocument struct {
	Model          string                  `toml:"model"`
	Protocol       domain.EndpointProtocol `toml:"protocol,omitempty"`
	ModelProvider  string                  `toml:"model_provider,omitempty"`
	Authentication domain.Authentication   `toml:"authentication,omitempty"`
}

type clientDocument struct {
	Enabled           bool                    `toml:"enabled"`
	Model             string                  `toml:"model,omitempty"`
	Protocol          domain.EndpointProtocol `toml:"protocol,omitempty"`
	ModelProvider     string                  `toml:"model_provider,omitempty"`
	Authentication    domain.Authentication   `toml:"authentication,omitempty"`
	Executable        string                  `toml:"executable,omitempty"`
	Targets           []string                `toml:"targets,omitempty"`
	CredentialCommand string                  `toml:"credential_command,omitempty"`
	Retries           *retriesDocument        `toml:"retries,omitempty,inline"`
	Forwarding        *forwardingDocument     `toml:"forwarding,omitempty,inline"`
}

type retriesDocument struct {
	Request *uint64 `toml:"request,omitempty"`
	Stream  *uint64 `toml:"stream,omitempty"`
}

// forwardingDocument records a client's forwarding destination and the
// upstream that its selection reached when the destination was set, so that a
// later change of upstream cannot rebind the destination silently.
type forwardingDocument struct {
	Endpoint string                  `toml:"endpoint"`
	Upstream string                  `toml:"upstream"`
	Protocol domain.EndpointProtocol `toml:"protocol"`
}

// UndefinedFieldError is a field that schema 8 does not define, such as a
// retired `order`, `purpose` or `lifecycle`, and the line that holds it. The
// field's path leaves out an inline table that holds it.
type UndefinedFieldError struct {
	Field string
	Line  int
}

// Error names the field, its line and the schema.
func (e *UndefinedFieldError) Error() string {
	return fmt.Sprintf("schema %d does not define the field %s, on line %d", domain.ConfigVersion, e.Field, e.Line)
}

// MissingFieldError is a field that schema 8 requires and the file leaves
// out, such as the endpoint of a client's forwarding.
type MissingFieldError struct{ Field string }

// Error names the field and the schema.
func (e *MissingFieldError) Error() string {
	return fmt.Sprintf("schema %d requires the field %s", domain.ConfigVersion, e.Field)
}

// readVersion reads only the document's version.
func readVersion(data []byte) (int, error) {
	var header struct {
		Version int `toml:"version"`
	}
	err := toml.Unmarshal(data, &header)
	return header.Version, err
}

// decodeDocument decodes data strictly: a field that schema 8 does not define
// is refused by its name.
func decodeDocument(data []byte) (document, error) {
	var decoded document
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&decoded)
	if strict, ok := errors.AsType[*toml.StrictMissingError](err); ok && len(strict.Errors) > 0 {
		line, _ := strict.Errors[0].Position()
		field := strings.Join(strict.Errors[0].Key(), ".")
		return document{}, domain.NewEntryError(fmt.Sprintf("the field `%s` on line %d", field, line), &UndefinedFieldError{Field: field, Line: line})
	}
	return decoded, err
}

// config builds the configuration that the document describes, in its
// display order. A repeated provider, model or family, or a model reference
// that does not parse, is refused.
func (d document) config(admission domain.Admission) (domain.Config, error) {
	cfg := domain.NewConfig(admission)
	cfg.Version = d.Version
	for _, provider := range d.Providers {
		entry := domain.Provider{Label: provider.Label, Endpoints: domain.Endpoints(provider.Endpoints)}
		if provider.Balance != nil {
			entry.BalanceProbe = &domain.BalanceProbe{Kind: provider.Balance.Kind, BaseURL: provider.Balance.BaseURL}
		}
		if err := cfg.AppendProvider(provider.ID, entry); err != nil {
			return domain.Config{}, domain.NewEntryError("provider `"+provider.ID+"`", err)
		}
		for _, model := range provider.Models {
			if err := cfg.AppendModel(domain.Model{
				Provider: provider.ID, ID: model.ID, Family: model.Family, Label: model.Label,
				Protocols: model.Protocols, Deprecated: model.Deprecated,
			}); err != nil {
				return domain.Config{}, domain.NewEntryError("model `"+provider.ID+"/"+model.ID+"`", err)
			}
		}
	}
	for _, family := range d.Families {
		if err := cfg.AppendFamily(family.ID, domain.Family{Label: family.Label}); err != nil {
			return domain.Config{}, domain.NewEntryError("family `"+family.ID+"`", err)
		}
	}
	for client, recommendation := range d.Recommendations {
		entry := "the `" + client + "` recommendation"
		primary, err := recommendation.Primary.selection(client)
		if err != nil {
			return domain.Config{}, domain.NewEntryError(entry, err)
		}
		decoded := domain.ClientRecommendation{Primary: primary}
		for _, alternative := range recommendation.Alternatives {
			selection, err := alternative.selection(client)
			if err != nil {
				return domain.Config{}, domain.NewEntryError(entry, err)
			}
			decoded.Alternatives = append(decoded.Alternatives, selection)
		}
		cfg.Recommendations[client] = decoded
	}
	for client, declared := range d.Clients {
		binding, err := declared.binding(client)
		if err != nil {
			return domain.Config{}, err
		}
		cfg.Clients[client] = binding
	}
	return cfg, nil
}

func (selection selectionDocument) selection(client string) (domain.ClientSelection, error) {
	ref, err := domain.ParseModelRef(selection.Model)
	if err != nil {
		return domain.ClientSelection{}, fmt.Errorf("the %s recommendation: %w", client, err)
	}
	return domain.ClientSelection{Model: ref, Protocol: selection.Protocol, ModelProvider: selection.ModelProvider, Authentication: selection.Authentication}, nil
}

func (declared clientDocument) binding(client string) (domain.ClientBinding, error) {
	binding := domain.ClientBinding{
		Enabled: declared.Enabled, Protocol: declared.Protocol, ModelProvider: declared.ModelProvider,
		Authentication: declared.Authentication, Executable: declared.Executable, Targets: declared.Targets,
		CredentialCommand: declared.CredentialCommand,
	}
	if declared.Model != "" {
		ref, err := domain.ParseModelRef(declared.Model)
		if err != nil {
			return domain.ClientBinding{}, fmt.Errorf("client %q: %w", client, err)
		}
		binding.Model = ref
	}
	if declared.Retries != nil {
		binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: declared.Retries.Request, StreamMaxRetries: declared.Retries.Stream}
	}
	if forwarding := declared.Forwarding; forwarding != nil {
		// A table without an endpoint would forward nothing, and the next
		// write would drop it.
		if forwarding.Endpoint == "" {
			return domain.ClientBinding{}, &MissingFieldError{Field: "clients." + client + ".forwarding.endpoint"}
		}
		binding.ForwardingEndpoint, binding.ForwardingUpstreamIdentity, binding.ForwardingProtocol = forwarding.Endpoint, forwarding.Upstream, forwarding.Protocol
	}
	return binding, nil
}

// newDocument describes cfg in its display order.
func newDocument(cfg domain.Config) document {
	described := document{Version: cfg.Version}
	for _, id := range cfg.ProviderIDs() {
		provider := cfg.Providers[id]
		entry := providerDocument{ID: id, Label: provider.Label, Endpoints: endpointsDocument(provider.Endpoints)}
		if provider.BalanceProbe != nil {
			entry.Balance = &balanceDocument{Kind: provider.BalanceProbe.Kind, BaseURL: provider.BalanceProbe.BaseURL}
		}
		for _, ref := range cfg.ProviderModelRefs(id) {
			model := cfg.Models[ref]
			protocols := model.Protocols
			if protocols == nil {
				protocols = map[domain.EndpointProtocol][]domain.Capability{}
			}
			entry.Models = append(entry.Models, modelDocument{
				ID: model.ID, Family: model.Family, Label: model.Label, Protocols: protocols, Deprecated: model.Deprecated,
			})
		}
		described.Providers = append(described.Providers, entry)
	}
	for _, id := range cfg.FamilyIDs() {
		described.Families = append(described.Families, familyDocument{ID: id, Label: cfg.Families[id].Label})
	}
	if len(cfg.Recommendations) > 0 {
		described.Recommendations = make(map[string]recommendationDocument, len(cfg.Recommendations))
		for client, recommendation := range cfg.Recommendations {
			entry := recommendationDocument{Primary: newSelectionDocument(recommendation.Primary)}
			for _, alternative := range recommendation.Alternatives {
				entry.Alternatives = append(entry.Alternatives, newSelectionDocument(alternative))
			}
			described.Recommendations[client] = entry
		}
	}
	if len(cfg.Clients) > 0 {
		described.Clients = make(map[string]clientDocument, len(cfg.Clients))
		for client, binding := range cfg.Clients {
			described.Clients[client] = newClientDocument(binding)
		}
	}
	return described
}

func newSelectionDocument(selection domain.ClientSelection) selectionDocument {
	return selectionDocument{Model: selection.Model.String(), Protocol: selection.Protocol, ModelProvider: selection.ModelProvider, Authentication: selection.Authentication}
}

func newClientDocument(binding domain.ClientBinding) clientDocument {
	declared := clientDocument{
		Enabled: binding.Enabled, Model: binding.Model.String(), Protocol: binding.Protocol, ModelProvider: binding.ModelProvider,
		Authentication: binding.Authentication, Executable: binding.Executable, Targets: binding.Targets,
		CredentialCommand: binding.CredentialCommand,
	}
	if policy := binding.RetryPolicy; policy != nil {
		declared.Retries = &retriesDocument{Request: policy.RequestMaxRetries, Stream: policy.StreamMaxRetries}
	}
	if binding.ForwardingEndpoint != "" {
		declared.Forwarding = &forwardingDocument{Endpoint: binding.ForwardingEndpoint, Upstream: binding.ForwardingUpstreamIdentity, Protocol: binding.ForwardingProtocol}
	}
	return declared
}

// encodeDocument writes the document as TOML, which separates its tables
// with one blank line, in the layout of the repository's TOML formatter. The
// recommendations and clients tables hold only one table per client, so
// their own empty headers, which TOML implies, are left out.
func encodeDocument(described document) ([]byte, error) {
	data, err := toml.Marshal(described)
	if err != nil {
		return nil, err
	}
	lines := slices.DeleteFunc(strings.Split(string(data), "\n"), func(line string) bool {
		return line == "[recommendations]" || line == "[clients]"
	})
	for index, line := range lines {
		lines[index] = spaceInlineTables(line)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// spaceInlineTables writes one space inside the braces of each inline table
// on line that is not empty, as taplo formats one, and leaves strings as they
// are. The encoder writes no multi-line string.
func spaceInlineTables(line string) string {
	var spaced strings.Builder
	var quote byte
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case quote == '"' && char == '\\' && index+1 < len(line):
			spaced.WriteByte(char)
			index++
			char = line[index]
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '"' || char == '\'':
			quote = char
		case char == '{' && index+1 < len(line) && line[index+1] != '}':
			spaced.WriteString("{ ")
			continue
		case char == '}' && index > 0 && line[index-1] != '{':
			spaced.WriteString(" }")
			continue
		}
		spaced.WriteByte(char)
	}
	return spaced.String()
}
