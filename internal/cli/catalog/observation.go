package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
)

type catalogModel struct {
	ID     string            `json:"id"`
	State  catalogEntryState `json:"state"`
	Routes []catalogRoute    `json:"routes,omitempty"`
}

type catalogRoute struct {
	ID            string                     `json:"id"`
	Model         string                     `json:"model"`
	UpstreamModel string                     `json:"upstream_model"`
	Capabilities  []configuration.Capability `json:"capabilities"`
	State         catalogEntryState          `json:"state,omitempty"`
}

type catalogSource struct {
	Protocol configuration.EndpointProtocol `json:"protocol"`
	Endpoint string                         `json:"endpoint"`
}

type catalogObservation struct {
	ObservationID   string         `json:"observation_id,omitempty"`
	Account         string         `json:"account"`
	Label           string         `json:"label"`
	Source          catalogSource  `json:"source"`
	SecretAvailable bool           `json:"secret_available"`
	Status          catalogStatus  `json:"status"`
	Models          []catalogModel `json:"models"`
	MissingRoutes   []catalogRoute `json:"missing_routes"`
}

type catalogStatus string

type catalogEntryState string

const (
	catalogOK                    catalogStatus = "ok"
	catalogEndpointUnavailable   catalogStatus = "catalog_endpoint_unavailable"
	catalogCredentialUnavailable catalogStatus = "credential_backend_failed"
	catalogTokenUnavailable      catalogStatus = "token_unavailable"
	catalogRequestFailed         catalogStatus = "request_failed"

	catalogCandidate               catalogEntryState = "candidate"
	catalogAdmitted                catalogEntryState = "admitted"
	catalogRequalificationRequired catalogEntryState = "requalification_required"
)

type catalogOutput struct {
	Observations []catalogObservation `json:"observations"`
}

func discoverCatalog(ctx context.Context, deps Dependencies, cfg configuration.Config) catalogOutput {
	result := catalogOutput{Observations: []catalogObservation{}}
	for _, accountName := range slices.Sorted(maps.Keys(cfg.Accounts)) {
		account := cfg.Accounts[accountName]
		sources := catalogSources(account)
		if len(sources) == 0 {
			result.Observations = append(result.Observations, catalogObservation{
				Account: accountName, Label: account.Label, Status: catalogEndpointUnavailable,
				Models: []catalogModel{}, MissingRoutes: []catalogRoute{},
			})
			continue
		}
		secretAvailable, observationErr := deps.Secrets.Exists(accountName)
		var token string
		if observationErr == nil && secretAvailable {
			token, observationErr = deps.Secrets.Get(accountName)
		}
		for _, source := range sources {
			entry := catalogObservation{
				Account: accountName, Label: account.Label, Source: source,
				SecretAvailable: secretAvailable, Models: []catalogModel{}, MissingRoutes: []catalogRoute{},
			}
			switch {
			case observationErr != nil:
				entry.Status = catalogCredentialUnavailable
			case !entry.SecretAvailable:
				entry.Status = catalogTokenUnavailable
			default:
				ids, endpoint, err := FetchIDs(ctx, deps.HTTP, source.Endpoint, source.Protocol, token)
				if err != nil {
					entry.Status = catalogRequestFailed
					break
				}
				entry.Status = catalogOK
				entry.Source.Endpoint = endpoint
				entry.Models, entry.MissingRoutes = catalogDifference(cfg, accountName, source.Protocol, ids)
				entry.ObservationID = catalogObservationID(accountName, source.Protocol, endpoint, ids)
			}
			result.Observations = append(result.Observations, entry)
		}
	}
	return result
}

func catalogSources(account configuration.Account) []catalogSource {
	protocols := []configuration.EndpointProtocol{
		configuration.ProtocolAnthropic,
		configuration.ProtocolOpenAIChatCompletions,
		configuration.ProtocolOpenAIResponses,
	}
	sources := make([]catalogSource, 0, len(protocols))
	for _, protocol := range protocols {
		if endpoint := strings.TrimRight(account.Endpoints.For(protocol), "/"); endpoint != "" {
			sources = append(sources, catalogSource{Protocol: protocol, Endpoint: endpoint})
		}
	}
	return sources
}

func catalogDifference(cfg configuration.Config, account string, protocol configuration.EndpointProtocol, ids []string) ([]catalogModel, []catalogRoute) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	observed := make(map[string]bool, len(ids))
	for _, id := range ids {
		observed[id] = true
	}

	routesByModel := make(map[string][]catalogRoute)
	missing := []catalogRoute{}
	for _, routeID := range cfg.RouteIDs() {
		route := cfg.Routes[routeID]
		capabilities, admitted := route.Interfaces[protocol]
		if route.Account != account || !admitted {
			continue
		}
		capabilities = slices.Clone(capabilities)
		slices.Sort(capabilities)
		entry := catalogRoute{
			ID: routeID, Model: route.Model, UpstreamModel: route.UpstreamModelID(),
			Capabilities: capabilities,
		}
		routesByModel[entry.UpstreamModel] = append(routesByModel[entry.UpstreamModel], entry)
		if !observed[entry.UpstreamModel] {
			entry.State = catalogRequalificationRequired
			missing = append(missing, entry)
		}
	}

	models := make([]catalogModel, 0, len(ids))
	for _, id := range ids {
		routes := routesByModel[id]
		state := catalogCandidate
		if len(routes) > 0 {
			state = catalogAdmitted
		}
		models = append(models, catalogModel{ID: id, State: state, Routes: routes})
	}
	return models, missing
}

func catalogObservationID(account string, protocol configuration.EndpointProtocol, endpoint string, ids []string) string {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	parts := append([]string{account, string(protocol), endpoint}, ids...)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\x00"))))
}

func observationKey(account string, protocol configuration.EndpointProtocol) string {
	return account + "\x00" + string(protocol)
}

// FetchIDs fetches and parses the model catalogue exposed by one exact wire protocol.
func FetchIDs(parent context.Context, client HTTPDoer, endpoint string, protocol configuration.EndpointProtocol, token string) ([]string, string, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	request, err := credential.ModelCatalogRequest(ctx, endpoint, protocol, token)
	if err != nil {
		return nil, "", err
	}
	response, err := credential.DoProbe(client, request)
	if err != nil {
		return nil, request.URL.String(), err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, request.URL.String(), fmt.Errorf("model catalog endpoint returned HTTP %d", response.StatusCode)
	}
	const maximumCatalogBytes = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumCatalogBytes+1))
	if err != nil {
		return nil, request.URL.String(), fmt.Errorf("model catalog response could not be read completely")
	}
	if len(body) > maximumCatalogBytes {
		return nil, request.URL.String(), fmt.Errorf("model catalog response exceeds the %d-byte limit", maximumCatalogBytes)
	}
	ids, err := ParseIDs(body)
	return ids, request.URL.String(), err
}

// ParseIDs accepts unambiguous common model item shapes and preserves exact IDs.
func ParseIDs(data []byte) ([]string, error) {
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("model catalog response is missing the data field")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(payload.Data, &items); err != nil {
		return nil, fmt.Errorf("model catalog response data field is not an array")
	}
	ids := []string{}
	seen := map[string]bool{}
	for index, item := range items {
		id, err := parseCatalogID(item)
		if err != nil {
			return nil, fmt.Errorf("model catalog item %d: %w", index, err)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func parseCatalogID(data json.RawMessage) (string, error) {
	var direct string
	if err := json.Unmarshal(data, &direct); err == nil {
		if direct = strings.TrimSpace(direct); direct == "" {
			return "", fmt.Errorf("identifier is empty")
		}
		return direct, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return "", fmt.Errorf("expected a string or object identifier")
	}
	identifiers := map[string]bool{}
	for _, key := range []string{"id", "model", "name"} {
		raw, exists := object[key]
		if !exists {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("%s is not a string", key)
		}
		if value = strings.TrimSpace(value); value == "" {
			return "", fmt.Errorf("%s is empty", key)
		}
		identifiers[value] = true
	}
	if len(identifiers) == 0 {
		return "", fmt.Errorf("identifier field is missing")
	}
	if len(identifiers) > 1 {
		return "", fmt.Errorf("identifier fields contradict each other")
	}
	for identifier := range identifiers {
		return identifier, nil
	}
	panic("unreachable")
}
