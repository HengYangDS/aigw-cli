// Package configuration owns AIGW's complete configuration domain: its
// schema, validation, persistence, and token-free interchange format.
package configuration

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"regexp"
	"sort"
)

// Authentication identifies which boundary owns credentials for a profile.
type Authentication string

const (
	// ConfigVersion is the only configuration schema version accepted by this build.
	ConfigVersion = 3
	// ClientClaude identifies the admitted Claude Code client.
	ClientClaude = "claude"
	// ClientCodex identifies the admitted Codex client family.
	ClientCodex = "codex"
	// ClientHermes identifies the independently configured Hermes Agent.
	ClientHermes = "hermes"
	// ModelProviderAIGW is the stable provider identifier written into AIGW-owned Codex projections.
	ModelProviderAIGW = "aigw"

	// AuthenticationAccountToken selects an AIGW-managed account token.
	AuthenticationAccountToken Authentication = "account-token"
	// AuthenticationClientNative delegates authentication to the selected client's native credential chain.
	AuthenticationClientNative Authentication = "client-native"
)

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var modelProviderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// Config is the typed source of truth for accounts, profiles, routes, and client bindings.
type Config struct {
	Version           int                      `json:"version"                      toml:"version"`
	Accounts          map[string]Account       `json:"accounts,omitempty"           toml:"accounts,omitempty"`
	Profiles          map[string]Profile       `json:"profiles"                     toml:"profiles"`
	Routes            Routes                   `json:"routes"                       toml:"routes"`
	RecommendedRoutes Routes                   `json:"recommended_routes,omitempty" toml:"recommended_routes,omitempty"`
	Clients           map[string]ClientBinding `json:"adapters,omitempty"           toml:"adapters,omitempty"`
}

// Account defines one provider capability and its protocol endpoints without containing credentials.
type Account struct {
	ID           string        `json:"id,omitempty"            toml:"-"`
	Label        string        `json:"label"                   toml:"label"`
	Endpoints    Endpoints     `json:"endpoints"               toml:"endpoints"`
	AccountProbe *AccountProbe `json:"account_probe,omitempty" toml:"account_probe,omitempty"`
}

// Profile binds one account, client, model, and authentication owner into a selectable route target.
type Profile struct {
	ID             string           `json:"id,omitempty"             toml:"-"`
	Label          string           `json:"label"                    toml:"label"`
	Purpose        string           `json:"purpose,omitempty"        toml:"purpose,omitempty"`
	Account        string           `json:"account"                  toml:"account"`
	Client         string           `json:"client"                   toml:"client"`
	Model          string           `json:"model"                    toml:"model"`
	Protocol       EndpointProtocol `json:"protocol,omitempty" toml:"protocol,omitempty"`
	ModelProvider  string           `json:"model_provider,omitempty" toml:"model_provider,omitempty"`
	Authentication Authentication   `json:"authentication,omitempty" toml:"authentication,omitempty"`
}

// Runtime is the resolved, immutable input used to project or invoke one client profile.
type Runtime struct {
	ProfileID         string           `json:"profile_id"`
	ProfileLabel      string           `json:"profile_label"`
	AccountID         string           `json:"account_id"`
	AccountLabel      string           `json:"account_label"`
	Client            string           `json:"client"`
	Endpoint          string           `json:"endpoint"`
	Protocol          EndpointProtocol `json:"protocol"`
	Model             string           `json:"model,omitempty"`
	ModelProvider     string           `json:"model_provider"`
	Authentication    Authentication   `json:"authentication"`
	CredentialCommand string           `json:"-"`
}

// RequiresAccountToken reports whether the selected Profile uses Account Token
// authentication on the wire. The zero value preserves the ordinary Account
// Token behavior for Profiles created before authentication was explicit.
func (runtime Runtime) RequiresAccountToken() bool {
	return runtime.Authentication == "" || runtime.Authentication == AuthenticationAccountToken
}

// CredentialProjectionFingerprint hashes the client, Account and endpoint identities.
// It detects stale credential projections; it is not a secret or caller authorization.
func (runtime Runtime) CredentialProjectionFingerprint(client string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(client+"\x00"+runtime.AccountID+"\x00"+runtime.Endpoint)))
}

// AccountProbe declares an optional provider-owned diagnostic API independently from inference traffic.
type AccountProbe struct {
	Kind    string `json:"kind"     toml:"kind"`
	BaseURL string `json:"base_url" toml:"base_url"`
}

// Endpoints declares the protocol-specific upstream URLs offered by an account.
type Endpoints struct {
	OpenAIChatCompletions string `json:"openai_chat_completions,omitempty" toml:"openai_chat_completions,omitempty"`
	OpenAIResponses       string `json:"openai_responses,omitempty" toml:"openai_responses,omitempty"`
	Anthropic             string `json:"anthropic,omitempty"        toml:"anthropic,omitempty"`
}

// Routes maps each admitted client to its selected Profile. There is no global
// fallback because a client-scoped Profile cannot represent another client's
// protocol or model.
type Routes map[string]string

// NewConfig returns an empty configuration with all collection invariants initialized.
func NewConfig() Config {
	return Config{Version: ConfigVersion, Accounts: map[string]Account{}, Profiles: map[string]Profile{}, Routes: Routes{}, RecommendedRoutes: Routes{}, Clients: map[string]ClientBinding{}}
}

// Clone returns an independent configuration value. Config is the semantic
// owner of this copy operation so mutation workflows do not reproduce its
// nested map and slice shape.
func (c Config) Clone() Config {
	out := NewConfig()
	out.Version = c.Version
	for name, account := range c.Accounts {
		if account.AccountProbe != nil {
			probe := *account.AccountProbe
			account.AccountProbe = &probe
		}
		out.Accounts[name] = account
	}
	maps.Copy(out.Profiles, c.Profiles)
	maps.Copy(out.Routes, c.Routes)
	maps.Copy(out.RecommendedRoutes, c.RecommendedRoutes)
	for name, adapter := range c.Clients {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		out.Clients[name] = adapter
	}
	return out
}

// ProfileIDs returns the stable lexical order used by every CLI projection.
func (c Config) ProfileIDs() []string {
	ids := make([]string, 0, len(c.Profiles))
	for id := range c.Profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ClientForProfile returns the canonical client declared by a named
// profile. Callers must not infer client scope from models, endpoints, routes,
// or profile names when this declaration is absent.
func (c Config) ClientForProfile(name string) (string, error) {
	profile, ok := c.Profiles[name]
	if !ok {
		return "", fmt.Errorf("unknown profile %q", name)
	}
	if !IsAdmittedClient(profile.Client) {
		return "", fmt.Errorf("profile %q does not declare exactly one admitted client", name)
	}
	return profile.Client, nil
}

// ResolveAccount accepts either an Account ID or a Profile ID and returns the
// referenced Account with its map identity populated.
func (c Config) ResolveAccount(reference string) (string, Account, error) {
	if account, ok := c.Accounts[reference]; ok {
		account.ID = reference
		return reference, account, nil
	}
	profile, ok := c.Profiles[reference]
	if !ok {
		return "", Account{}, fmt.Errorf("unknown account or profile %q", reference)
	}
	account, ok := c.Accounts[profile.Account]
	if !ok {
		return "", Account{}, fmt.Errorf("profile %q references unknown account %q", reference, profile.Account)
	}
	account.ID = profile.Account
	return profile.Account, account, nil
}

// FirstProfileForClient returns the first stable profile that can resolve the
// requested client's model or endpoint without changing a route.
func (c Config) FirstProfileForClient(client string) string {
	for _, id := range c.ProfileIDs() {
		profile := c.Profiles[id]
		if profile.Client != "" && profile.Client != client {
			continue
		}
		if profile.Model != "" {
			return id
		}
		if account, ok := c.Accounts[profile.Account]; ok {
			account.ID = profile.Account
			if _, err := account.EndpointFor(client); err == nil {
				return id
			}
		}
	}
	return ""
}

// EnabledClientIDs returns the stable client scope explicitly enabled by this configuration.
func (c Config) EnabledClientIDs() []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range AdmittedClientIDs() {
		if c.Clients[client].Enabled {
			clients = append(clients, client)
		}
	}
	return clients
}

// RouteUsesAccount reports whether the resolved route for client selects the
// given Account.
func (c Config) RouteUsesAccount(client, accountID string) bool {
	runtime, err := c.ResolveRuntime(client, "")
	return err == nil && runtime.AccountID != "" && runtime.AccountID == accountID
}

// RoutedAccountIDs returns the stable set of Accounts selected by resolvable
// admitted-client Routes. Catalogue Accounts that no active Route selects are
// capabilities available for later connection, not current runtime
// dependencies.
func (c Config) RoutedAccountIDs() []string {
	selected := map[string]bool{}
	for _, client := range AdmittedClientIDs() {
		runtime, err := c.ResolveRuntime(client, "")
		if err == nil && runtime.AccountID != "" {
			selected[runtime.AccountID] = true
		}
	}
	accountIDs := make([]string, 0, len(selected))
	for accountID := range selected {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Strings(accountIDs)
	return accountIDs
}

// RequiredAccountTokenIDs returns the stable set of Accounts whose Tokens are
// required by enabled, selected Routes. Client-native Profiles remain client
// credential concerns and therefore never create an AIGW Token requirement.
func (c Config) RequiredAccountTokenIDs() []string {
	required := map[string]bool{}
	for _, client := range c.EnabledClientIDs() {
		runtime, err := c.ResolveRuntime(client, "")
		if err == nil && runtime.AccountID != "" && runtime.RequiresAccountToken() {
			required[runtime.AccountID] = true
		}
	}
	accountIDs := make([]string, 0, len(required))
	for accountID := range required {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Strings(accountIDs)
	return accountIDs
}

// SelectRoutesForConnectedAccounts preserves the complete capability catalogue
// while filling unselected routes whose authentication is currently usable. A
// client-native Profile is usable without an AIGW Account Token; an
// account-token Profile is usable only when its Account is connected. The
// usable recommendation wins, followed by a compatible Profile of the same
// model, then lexical Profile order. Existing selections are never replaced.
func (c Config) SelectRoutesForConnectedAccounts(accountIDs []string, clients ...string) (Config, error) {
	selected := c.Clone()
	connected := make(map[string]bool, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := selected.Accounts[accountID]; !ok {
			return Config{}, fmt.Errorf("unknown account %q", accountID)
		}
		connected[accountID] = true
	}
	if len(clients) == 0 {
		clients = AdmittedClientIDs()
	}
	for _, client := range clients {
		if selected.Routes[client] != "" {
			continue
		}
		profileID := selected.profileForAvailableAuthentication(client, connected)
		if profileID == "" {
			continue
		}
		selected.Routes[client] = profileID
	}
	return selected, nil
}

func (c Config) profileForAvailableAuthentication(client string, connected map[string]bool) string {
	preferredModel := ""
	if recommendation := c.RecommendedRoutes[client]; recommendation != "" {
		runtime, err := c.ResolveRuntime(client, recommendation)
		if err != nil {
			return ""
		}
		if !runtime.RequiresAccountToken() || connected[runtime.AccountID] {
			return recommendation
		}
		preferredModel = runtime.Model
	}
	replacement := ""
	for _, profileID := range c.ProfileIDs() {
		runtime, err := c.ResolveRuntime(client, profileID)
		if err != nil || runtime.RequiresAccountToken() && !connected[runtime.AccountID] {
			continue
		}
		profile := c.Profiles[profileID]
		if profile.Model != "" {
			if preferredModel != "" && profile.Model == preferredModel {
				return profileID
			}
			if replacement == "" {
				replacement = profileID
			}
		}
	}
	return replacement
}

// ResolveRuntime resolves one client route to its validated account and profile runtime.
func (c Config) ResolveRuntime(client, explicitProfile string) (Runtime, error) {
	binding := c.Clients[client]
	name := explicitProfile
	if name == "" {
		name = binding.Profile
		if name == "" {
			name = c.Routes[client]
		}
		if name == "" {
			return Runtime{}, &RuntimeRouteUnselectedError{Client: client}
		}
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown profile %q", name)
	}
	profile.ID = name
	if profile.Client != "" && profile.Client != client {
		return Runtime{}, &RuntimeProfileClientMismatchError{ProfileID: name, ExpectedClient: profile.Client, ActualClient: client}
	}
	account, ok := c.Accounts[profile.Account]
	if !ok {
		return Runtime{}, &RuntimeProfileUnknownAccountError{ProfileID: name, AccountID: profile.Account}
	}
	account.ID = profile.Account
	spec, admitted := ClientSpecFor(client)
	if !admitted {
		return Runtime{}, fmt.Errorf("unknown client %q", client)
	}
	endpoint, protocol, err := spec.ResolveEndpoint(account, selectedProtocol(binding, profile))
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{
		ProfileID:         name,
		ProfileLabel:      profile.Label,
		AccountID:         account.ID,
		AccountLabel:      account.Label,
		Client:            client,
		Endpoint:          endpoint,
		Protocol:          protocol,
		Model:             profile.Model,
		ModelProvider:     selectedModelProvider(client, binding, profile),
		Authentication:    selectedAuthentication(binding, profile),
		CredentialCommand: c.Clients[client].CredentialCommand,
	}, nil
}

func selectedProtocol(binding ClientBinding, profile Profile) EndpointProtocol {
	if binding.Protocol != "" {
		return binding.Protocol
	}
	return profile.Protocol
}

func selectedModelProvider(client string, binding ClientBinding, profile Profile) string {
	if binding.ModelProvider != "" {
		return binding.ModelProvider
	}
	return resolvedModelProvider(client, profile)
}

func selectedAuthentication(binding ClientBinding, profile Profile) Authentication {
	if binding.Authentication != "" {
		return binding.Authentication
	}
	return resolvedAuthentication(profile)
}

// EndpointFor returns the endpoint whose protocol is required by the requested admitted client.
func (account Account) EndpointFor(client string) (string, error) {
	spec, ok := ClientSpecFor(client)
	if !ok {
		return "", fmt.Errorf("unknown client %q", client)
	}
	endpoint, _, err := spec.ResolveEndpoint(account, "")
	return endpoint, err
}
