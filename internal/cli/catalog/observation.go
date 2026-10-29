package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"

	"aigw-cli/internal/domain"
)

type catalogModel struct {
	ID     string            `json:"id"`
	State  catalogEntryState `json:"state"`
	Routes []catalogRoute    `json:"routes,omitempty"`
}

type catalogRoute struct {
	ID            string                `json:"id"`
	Model         string                `json:"model"`
	UpstreamModel string                `json:"upstream_model"`
	Capabilities  []domain.Capability   `json:"capabilities"`
	Lifecycle     domain.RouteLifecycle `json:"lifecycle"`
	Evidence      catalogEvidence       `json:"evidence"`
}

type catalogSource struct {
	Protocol domain.EndpointProtocol `json:"protocol"`
	Endpoint string                  `json:"endpoint"`
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

type catalogEvidence string

const (
	catalogOK                    catalogStatus = "ok"
	catalogEndpointUnavailable   catalogStatus = "catalog_endpoint_unavailable"
	catalogCredentialUnavailable catalogStatus = "credential_backend_failed"
	catalogTokenUnavailable      catalogStatus = "token_unavailable"
	catalogRequestFailed         catalogStatus = "request_failed"

	catalogCandidate  catalogEntryState = "candidate"
	catalogAdmitted   catalogEntryState = "admitted"
	catalogDeprecated catalogEntryState = "deprecated"

	catalogObserved                catalogEvidence = "observed"
	catalogRequalificationRequired catalogEvidence = "requalification_required"
)

type catalogOutput struct {
	Observations []catalogObservation `json:"observations"`
}

func discoverCatalog(ctx context.Context, deps Dependencies, cfg domain.Config) catalogOutput {
	result := catalogOutput{Observations: []catalogObservation{}}
	for _, accountName := range cfg.CatalogAccountIDs() {
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

func catalogSources(account domain.Account) []catalogSource {
	protocols := []domain.EndpointProtocol{
		domain.ProtocolAnthropic,
		domain.ProtocolOpenAIChatCompletions,
		domain.ProtocolOpenAIResponses,
	}
	slices.SortFunc(protocols, domain.CompareProtocols)
	sources := make([]catalogSource, 0, len(protocols))
	for _, protocol := range protocols {
		if endpoint := strings.TrimRight(account.Endpoints.For(protocol), "/"); endpoint != "" {
			sources = append(sources, catalogSource{Protocol: protocol, Endpoint: endpoint})
		}
	}
	return sources
}

func catalogDifference(cfg domain.Config, account string, protocol domain.EndpointProtocol, ids []string) ([]catalogModel, []catalogRoute) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	observed := make(map[string]bool, len(ids))
	for _, id := range ids {
		observed[id] = true
	}

	routesByModel := make(map[string][]catalogRoute)
	missing := []catalogRoute{}
	for _, routeID := range cfg.CatalogRouteIDs() {
		route := cfg.Routes[routeID]
		capabilities, admitted := route.Interfaces[protocol]
		if route.Account != account || !admitted {
			continue
		}
		capabilities = slices.Clone(capabilities)
		slices.Sort(capabilities)
		entry := catalogRoute{
			ID: routeID, Model: route.Model, UpstreamModel: route.UpstreamModelID(),
			Capabilities: capabilities, Lifecycle: route.LifecycleState(), Evidence: catalogObserved,
		}
		routesByModel[entry.UpstreamModel] = append(routesByModel[entry.UpstreamModel], entry)
		if !observed[entry.UpstreamModel] {
			entry.Evidence = catalogRequalificationRequired
			missing = append(missing, entry)
		}
	}

	models := make([]catalogModel, 0, len(ids))
	for _, id := range ids {
		routes := routesByModel[id]
		state := catalogCandidate
		for _, route := range routes {
			if route.Lifecycle == domain.RouteAdmitted {
				state = catalogAdmitted
				break
			}
			state = catalogDeprecated
		}
		models = append(models, catalogModel{ID: id, State: state, Routes: routes})
	}
	slices.SortStableFunc(models, func(left, right catalogModel) int {
		if len(left.Routes) == 0 && len(right.Routes) > 0 {
			return 1
		}
		if len(right.Routes) == 0 && len(left.Routes) > 0 {
			return -1
		}
		if len(left.Routes) > 0 && len(right.Routes) > 0 {
			if rank := cfg.CompareModels(left.Routes[0].Model, right.Routes[0].Model); rank != 0 {
				return rank
			}
		}
		return strings.Compare(left.ID, right.ID)
	})
	return models, missing
}

func catalogObservationID(account string, protocol domain.EndpointProtocol, endpoint string, ids []string) string {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	parts := append([]string{account, string(protocol), endpoint}, ids...)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\x00"))))
}

func observationKey(account string, protocol domain.EndpointProtocol) string {
	return account + "\x00" + string(protocol)
}

// FetchIDs fetches and parses the model catalogue exposed by one exact wire protocol.
func FetchIDs(parent context.Context, client HTTPDoer, endpoint string, protocol domain.EndpointProtocol, token string) ([]string, string, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	request, err := domain.ModelCatalogRequest(ctx, endpoint, protocol, token)
	if err != nil {
		return nil, "", err
	}
	response, err := domain.DoProbe(client, request)
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
// The model list must be one array, and each item may name its identifier
// once: encoding/json would read null as an empty list and keep the last of
// repeated members, turning an ambiguous response into a confident observation.
func ParseIDs(data []byte) ([]string, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("model catalog response is not valid JSON")
	}
	members, err := catalogMembers(data, "data")
	if errors.Is(err, errNotObject) {
		return nil, fmt.Errorf("model catalog response is not a JSON object")
	}
	if err != nil {
		return nil, fmt.Errorf("model catalog response %w", err)
	}
	list, present := members["data"]
	if !present {
		return nil, fmt.Errorf("model catalog response is missing the data field")
	}
	var items []json.RawMessage
	if list = bytes.TrimSpace(list); len(list) == 0 || list[0] != '[' || json.Unmarshal(list, &items) != nil {
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

	members, err := catalogMembers(data, "id", "model", "name")
	if errors.Is(err, errNotObject) {
		return "", fmt.Errorf("expected a string or object identifier")
	}
	if err != nil {
		return "", err
	}
	identifiers := map[string]bool{}
	for _, key := range []string{"id", "model", "name"} {
		raw, exists := members[key]
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

var errNotObject = errors.New("not a JSON object")

// catalogMembers decodes one valid JSON object and returns the named members,
// refusing a repeated one. Other members are provider extensions; they are
// skipped unread, repeated or not.
func catalogMembers(data []byte, names ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, errNotObject
	}
	members := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, _ := token.(string)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if !slices.Contains(names, name) {
			continue
		}
		if _, repeated := members[name]; repeated {
			return nil, fmt.Errorf("repeats the %s member", name)
		}
		members[name] = value
	}
	return members, nil
}
