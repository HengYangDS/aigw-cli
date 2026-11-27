package store

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

// document is aigw.toml in schema 9: providers with their models, families,
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

// modelDocument is one model of a provider. ContextWindow is absent for a
// model whose window is the default, so that such a model is written as it
// was before the field existed. A file may hold the default all the same,
// which reads as no declaration (domain.DeclaredContextWindow); a pointer
// tells an absent field from a zero, which is refused.
type modelDocument struct {
	ID            string                                          `toml:"id"`
	Family        string                                          `toml:"family,omitempty"`
	Label         string                                          `toml:"label,omitempty"`
	Protocols     map[domain.EndpointProtocol][]domain.Capability `toml:"protocols,inline"`
	ContextWindow *int                                            `toml:"context_window,omitempty"`
	Deprecated    bool                                            `toml:"deprecated,omitempty"`
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
	Model    string                  `toml:"model"`
	Protocol domain.EndpointProtocol `toml:"protocol,omitempty"`
}

type clientDocument struct {
	Enabled           bool                    `toml:"enabled"`
	Model             string                  `toml:"model,omitempty"`
	Protocol          domain.EndpointProtocol `toml:"protocol,omitempty"`
	Program           string                  `toml:"program,omitempty"`
	Extensions        string                  `toml:"extensions,omitempty"`
	Targets           []string                `toml:"targets,omitempty"`
	CredentialCommand string                  `toml:"credential_command,omitempty"`
	Retries           *retriesDocument        `toml:"retries,omitempty,inline"`
	// Forwarding is the root of the forwarding service through which the
	// client's Responses traffic goes.
	Forwarding string `toml:"forwarding,omitempty"`
}

type retriesDocument struct {
	Request *uint64 `toml:"request,omitempty"`
	Stream  *uint64 `toml:"stream,omitempty"`
}

// UndefinedFieldError is a field that the schema does not define, such as a
// retired `order`, `purpose` or `lifecycle`, or a client's `executable`,
// `authentication` or `model_provider`, and the line that holds it. The field's path leaves
// out an inline table that holds it.
type UndefinedFieldError struct {
	Field string
	Line  int
}

// Error names the field, its line and the schema.
func (e *UndefinedFieldError) Error() string {
	return fmt.Sprintf("schema %d does not define the field %s, on line %d", domain.ConfigVersion, e.Field, e.Line)
}

// ForwardingTableError is a client's `forwarding` that holds a table, as the
// schema that the current one replaced wrote it, where the current schema
// holds the root of a forwarding service, and the line that holds it.
type ForwardingTableError struct {
	Client string
	Line   int
}

// Error names the field, its line and what the schema holds there. The edit
// that replaces the table is Edit, a fix of the file: until it is made, no
// command loads the configuration.
func (e *ForwardingTableError) Error() string {
	return fmt.Sprintf("the field clients.%s.forwarding on line %d holds a table, where schema %d holds the root of a forwarding service", e.Client, e.Line, domain.ConfigVersion)
}

// Edit is the edit that brings the field to the current schema, the same
// that the refusal of a file of the replaced schema names for each client.
func (e *ForwardingTableError) Edit() string { return domain.ForwardingEdit(e.Client) }

// readVersion reads only the document's version.
func readVersion(data []byte) (int, error) {
	var header struct {
		Version int `toml:"version"`
	}
	err := toml.Unmarshal(data, &header)
	return header.Version, err
}

// decodeDocument decodes data strictly: a field that the schema does not
// define is refused by its name, and a client's forwarding table of the
// schema that the current one replaced by the edit that replaces it.
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
	if mistyped, ok := errors.AsType[*toml.DecodeError](err); ok {
		if key := mistyped.Key(); len(key) == 3 && key[0] == "clients" && key[2] == "forwarding" && forwardingIsTable(data, key[1]) {
			line, _ := mistyped.Position()
			return document{}, domain.NewEntryError(fmt.Sprintf("the field `%s` on line %d", strings.Join(key, "."), line), &ForwardingTableError{Client: key[1], Line: line})
		}
	}
	return decoded, err
}

// forwardingIsTable reports whether data gives client a `forwarding` table.
// It reads data again only after the strict decoding failed there.
func forwardingIsTable(data []byte, client string) bool {
	var raw struct {
		Clients map[string]map[string]any `toml:"clients"`
	}
	if toml.Unmarshal(data, &raw) != nil {
		return false
	}
	_, table := raw.Clients[client]["forwarding"].(map[string]any)
	return table
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
			modelEntry := "model `" + provider.ID + "/" + model.ID + "`"
			described := domain.Model{
				Provider: provider.ID, ID: model.ID, Family: model.Family, Label: model.Label,
				Protocols: model.Protocols, Deprecated: model.Deprecated,
			}
			if model.ContextWindow != nil {
				if err := domain.ValidateContextWindow(*model.ContextWindow); err != nil {
					return domain.Config{}, domain.NewEntryError(modelEntry, fmt.Errorf("model %s: %w", described.Ref(), err))
				}
				described.ContextWindow = domain.DeclaredContextWindow(*model.ContextWindow)
			}
			if err := cfg.AppendModel(described); err != nil {
				return domain.Config{}, domain.NewEntryError(modelEntry, err)
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
	return domain.ClientSelection{Model: ref, Protocol: selection.Protocol}, nil
}

func (declared clientDocument) binding(client string) (domain.ClientBinding, error) {
	binding := domain.ClientBinding{
		Enabled: declared.Enabled, Protocol: declared.Protocol, Program: declared.Program, Extensions: declared.Extensions,
		Targets: declared.Targets, CredentialCommand: declared.CredentialCommand, ForwardingRoot: declared.Forwarding,
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
			declared := modelDocument{
				ID: model.ID, Family: model.Family, Label: model.Label, Protocols: protocols, Deprecated: model.Deprecated,
			}
			if window := domain.DeclaredContextWindow(model.Window()); window != 0 {
				declared.ContextWindow = &window
			}
			entry.Models = append(entry.Models, declared)
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
	return selectionDocument{Model: selection.Model.String(), Protocol: selection.Protocol}
}

func newClientDocument(binding domain.ClientBinding) clientDocument {
	declared := clientDocument{
		Enabled: binding.Enabled, Model: binding.Model.String(), Protocol: binding.Protocol,
		Program: binding.Program, Extensions: binding.Extensions, Targets: binding.Targets, CredentialCommand: binding.CredentialCommand,
		Forwarding: binding.ForwardingRoot,
	}
	if policy := binding.RetryPolicy; policy != nil {
		declared.Retries = &retriesDocument{Request: policy.RequestMaxRetries, Stream: policy.StreamMaxRetries}
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

// columnWidth is the widest line that a team file export writes on purpose:
// the width past which taplo, the formatter of the repository that ships the
// team file, expands an array.
const columnWidth = 100

// expandWideArrays expands each array of data, a team file, whose line is
// wider than columnWidth, so that an exported team file is the one that taplo
// keeps. aigw.toml keeps one line for each value.
func expandWideArrays(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		lines[index] = expandWideArray(line)
	}
	return []byte(strings.Join(lines, "\n"))
}

// expandWideArray writes the array value of a line wider than columnWidth
// with one element on each line, indented two spaces and followed by a comma,
// as taplo expands one. A narrower line, a line whose value is no array, and
// an inline table, which TOML keeps on one line, stay as they are.
func expandWideArray(line string) string {
	if utf8.RuneCountInString(line) <= columnWidth {
		return line
	}
	key, value, found := strings.Cut(line, " = [")
	if !found || !strings.HasSuffix(value, "]") || strings.TrimLeft(key, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" {
		return line
	}
	elements := arrayElements(strings.TrimSuffix(value, "]"))
	if len(elements) == 0 {
		return line
	}
	var expanded strings.Builder
	expanded.WriteString(key + " = [\n")
	for _, element := range elements {
		expanded.WriteString("  " + element + ",\n")
	}
	expanded.WriteString("]")
	return expanded.String()
}

// arrayElements splits the inside of a one-line array at the commas outside
// strings, inline tables and nested arrays, or returns nothing for brackets
// that do not balance.
func arrayElements(inside string) []string {
	var elements []string
	var quote byte
	depth, start := 0, 0
	for index := 0; index < len(inside); index++ {
		char := inside[index]
		switch {
		case quote == '"' && char == '\\':
			index++
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '"' || char == '\'':
			quote = char
		case char == '{' || char == '[':
			depth++
		case char == '}' || char == ']':
			depth--
		case char == ',' && depth == 0:
			elements = append(elements, strings.TrimSpace(inside[start:index]))
			start = index + 1
		}
	}
	if depth != 0 || quote != 0 {
		return nil
	}
	if last := strings.TrimSpace(inside[start:]); last != "" {
		elements = append(elements, last)
	}
	return elements
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
