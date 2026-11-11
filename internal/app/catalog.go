package app

import (
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
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
	"sync"
	"time"
)

// Label describes the status for a person.
func (status CatalogStatus) Label() string {
	switch status {
	case CatalogOK:
		return string(status)
	case CatalogEndpointUnavailable:
		return "No supported model catalog endpoint is configured"
	case CatalogTokenUnavailable:
		return "Token unavailable"
	case CatalogCredentialUnavailable:
		return "Credential backend failed; catalog was not queried"
	case CatalogRequestFailed:
		return "Catalog request failed; configuration was not changed"
	default:
		return string(status)
	}
}

// CatalogModel is one model that a provider catalog lists, with the Routes
// that use it.
type CatalogModel struct {
	ID     string            `json:"id"`
	State  CatalogEntryState `json:"state"`
	Models []ConfiguredModel `json:"routes,omitempty"`
}

// ConfiguredModel is a configured model as one catalog observation sees it.
type ConfiguredModel struct {
	ID            string                `json:"id"`
	Family        string                `json:"model"`
	UpstreamModel string                `json:"upstream_model"`
	Capabilities  []domain.Capability   `json:"capabilities"`
	Lifecycle     domain.ModelLifecycle `json:"lifecycle"`
	Evidence      CatalogEvidence       `json:"evidence"`
}

// CatalogSource is the endpoint and protocol that one observation read.
type CatalogSource struct {
	Protocol domain.EndpointProtocol `json:"protocol"`
	Endpoint string                  `json:"endpoint"`
}

// CatalogObservation is one Account's catalog for one protocol.
type CatalogObservation struct {
	ObservationID   string            `json:"observation_id,omitempty"`
	Provider        string            `json:"account"`
	Label           string            `json:"label"`
	Source          CatalogSource     `json:"source"`
	SecretAvailable bool              `json:"secret_available"`
	Status          CatalogStatus     `json:"status"`
	Models          []CatalogModel    `json:"models"`
	MissingModels   []ConfiguredModel `json:"missing_routes"`
	// Failure is why the catalog could not be read: the credential backend's
	// failure or the request's, such as a CatalogStatusError. Documents do
	// not carry it.
	Failure error `json:"-"`
}

// CatalogStatusError is a catalog request that its endpoint answered with a
// status other than success.
type CatalogStatusError struct{ Status int }

// Error names the status.
func (e *CatalogStatusError) Error() string {
	return fmt.Sprintf("model catalog endpoint returned HTTP %d", e.Status)
}

// ErrCatalogFormat marks a successful response that is not one complete,
// unambiguous model list, which the same request would not change.
var ErrCatalogFormat = errors.New("the response is not a model catalog")

// CatalogStatus is the outcome of one catalog observation.
type CatalogStatus string

// CatalogEntryState says whether a listed model is admitted, deprecated or a
// candidate for qualification.
type CatalogEntryState string

// CatalogEvidence says how a Route's catalog membership is known.
type CatalogEvidence string

// The catalog statuses, entry states and evidence kinds.
const (
	CatalogOK                    CatalogStatus = "ok"
	CatalogEndpointUnavailable   CatalogStatus = "catalog_endpoint_unavailable"
	CatalogCredentialUnavailable CatalogStatus = "credential_backend_failed"
	CatalogTokenUnavailable      CatalogStatus = "token_unavailable"
	CatalogRequestFailed         CatalogStatus = "request_failed"

	CatalogCandidate  CatalogEntryState = "candidate"
	CatalogAdmitted   CatalogEntryState = "admitted"
	CatalogDeprecated CatalogEntryState = "deprecated"

	CatalogObserved                CatalogEvidence = "observed"
	CatalogRequalificationRequired CatalogEvidence = "requalification_required"
)

// CatalogReport holds every catalog observation of one discovery.
type CatalogReport struct {
	Observations []CatalogObservation `json:"observations"`
}

// DiscoverCatalog reads each configured provider's catalog for each protocol
// it serves, without changing configuration. It reads the providers' Tokens
// one provider at a time, and then sends every distinct catalog request at
// once: protocols whose requests are the same share one read, and each read
// keeps its own 15-second bound, so the whole discovery takes one bound.
func DiscoverCatalog(ctx context.Context, secrets secret.Store, client HTTPDoer, cfg domain.Config) CatalogReport {
	result := CatalogReport{Observations: []CatalogObservation{}}
	var reads []*catalogRead
	pending := map[string]*catalogRead{}
	for _, providerID := range cfg.CatalogProviderIDs() {
		provider := cfg.Providers[providerID]
		sources := catalogSources(provider)
		if len(sources) == 0 {
			result.Observations = append(result.Observations, CatalogObservation{
				Provider: providerID, Label: provider.Label, Status: CatalogEndpointUnavailable,
				Models: []CatalogModel{}, MissingModels: []ConfiguredModel{},
			})
			continue
		}
		secretAvailable, observationErr := secrets.Exists(providerID)
		var token string
		if observationErr == nil && secretAvailable {
			token, observationErr = secrets.Get(providerID)
		}
		for _, source := range sources {
			entry := CatalogObservation{
				Provider: providerID, Label: provider.Label, Source: source,
				SecretAvailable: secretAvailable, Models: []CatalogModel{}, MissingModels: []ConfiguredModel{},
			}
			switch {
			case observationErr != nil:
				entry.Status, entry.Failure = CatalogCredentialUnavailable, observationErr
			case !secretAvailable:
				entry.Status = CatalogTokenUnavailable
			default:
				key := providerID + "\x00" + domain.CatalogRequestKey(source.Endpoint, source.Protocol)
				if pending[key] == nil {
					pending[key] = &catalogRead{source: source, token: token}
					reads = append(reads, pending[key])
				}
				pending[key].observations = append(pending[key].observations, len(result.Observations))
			}
			result.Observations = append(result.Observations, entry)
		}
	}
	var wait sync.WaitGroup
	for _, read := range reads {
		wait.Go(func() {
			read.ids, read.endpoint, read.err = fetchIDs(ctx, client, read.source.Endpoint, read.source.Protocol, read.token)
		})
	}
	wait.Wait()
	for _, read := range reads {
		for _, index := range read.observations {
			read.record(cfg, &result.Observations[index])
		}
	}
	return result
}

// catalogRead is one distinct catalog request, the observations that it
// answers, and its outcome.
type catalogRead struct {
	source       CatalogSource
	token        string
	observations []int
	ids          []string
	endpoint     string
	err          error
}

// record completes an observation that the read answers: a failure, or the
// models that the catalog lists and the configured models that it does not.
func (read *catalogRead) record(cfg domain.Config, entry *CatalogObservation) {
	if read.err != nil {
		entry.Status, entry.Failure = CatalogRequestFailed, read.err
		return
	}
	entry.Status = CatalogOK
	entry.Source.Endpoint = read.endpoint
	entry.Models, entry.MissingModels = catalogDifference(cfg, entry.Provider, entry.Source.Protocol, read.ids)
	entry.ObservationID = catalogObservationID(entry.Provider, entry.Source.Protocol, read.endpoint, read.ids)
}

func catalogSources(account domain.Provider) []CatalogSource {
	protocols := []domain.EndpointProtocol{
		domain.ProtocolAnthropic,
		domain.ProtocolChat,
		domain.ProtocolResponses,
	}
	slices.SortFunc(protocols, domain.CompareProtocols)
	sources := make([]CatalogSource, 0, len(protocols))
	for _, protocol := range protocols {
		if endpoint := strings.TrimRight(account.Endpoints.For(protocol), "/"); endpoint != "" {
			sources = append(sources, CatalogSource{Protocol: protocol, Endpoint: endpoint})
		}
	}
	return sources
}

func catalogDifference(cfg domain.Config, account string, protocol domain.EndpointProtocol, ids []string) ([]CatalogModel, []ConfiguredModel) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	observed := make(map[string]bool, len(ids))
	for _, id := range ids {
		observed[id] = true
	}

	routesByModel := make(map[string][]ConfiguredModel)
	missing := []ConfiguredModel{}
	for _, routeID := range cfg.CatalogModelIDs() {
		route := cfg.Models[routeID]
		capabilities, admitted := route.Protocols[protocol]
		if route.Provider != account || !admitted {
			continue
		}
		capabilities = slices.Clone(capabilities)
		slices.Sort(capabilities)
		entry := ConfiguredModel{
			ID: routeID, Family: route.Family, UpstreamModel: route.UpstreamModelID(),
			Capabilities: capabilities, Lifecycle: route.LifecycleState(), Evidence: CatalogObserved,
		}
		routesByModel[entry.UpstreamModel] = append(routesByModel[entry.UpstreamModel], entry)
		if !observed[entry.UpstreamModel] {
			entry.Evidence = CatalogRequalificationRequired
			missing = append(missing, entry)
		}
	}

	models := make([]CatalogModel, 0, len(ids))
	for _, id := range ids {
		routes := routesByModel[id]
		state := CatalogCandidate
		for _, route := range routes {
			if route.Lifecycle == domain.ModelAdmitted {
				state = CatalogAdmitted
				break
			}
			state = CatalogDeprecated
		}
		models = append(models, CatalogModel{ID: id, State: state, Models: routes})
	}
	slices.SortStableFunc(models, func(left, right CatalogModel) int {
		if len(left.Models) == 0 && len(right.Models) > 0 {
			return 1
		}
		if len(right.Models) == 0 && len(left.Models) > 0 {
			return -1
		}
		if len(left.Models) > 0 && len(right.Models) > 0 {
			if rank := cfg.CompareFamilies(left.Models[0].Family, right.Models[0].Family); rank != 0 {
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

// fetchIDs fetches and parses the model catalog exposed by one exact wire protocol.
func fetchIDs(parent context.Context, client HTTPDoer, endpoint string, protocol domain.EndpointProtocol, token string) ([]string, string, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	request, err := domain.ModelCatalogRequest(ctx, endpoint, protocol, token)
	if err != nil {
		return nil, "", err
	}
	response, err := probe.Send(client, request)
	if err != nil {
		return nil, request.URL.String(), err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, request.URL.String(), &CatalogStatusError{Status: response.StatusCode}
	}
	const maximumCatalogBytes = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumCatalogBytes+1))
	if err != nil {
		return nil, request.URL.String(), fmt.Errorf("model catalog response could not be read completely")
	}
	if len(body) > maximumCatalogBytes {
		return nil, request.URL.String(), fmt.Errorf("%w: it exceeds the %d-byte limit", ErrCatalogFormat, maximumCatalogBytes)
	}
	ids, err := parseIDs(body)
	if err != nil {
		return nil, request.URL.String(), fmt.Errorf("%w: %w", ErrCatalogFormat, err)
	}
	return ids, request.URL.String(), nil
}

// parseIDs accepts unambiguous common model item shapes and preserves exact IDs.
// The model list must be one array, and each item may name its identifier
// once: encoding/json would read null as an empty list and keep the last of
// repeated members, turning an ambiguous response into a confident observation.
func parseIDs(data []byte) ([]string, error) {
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
