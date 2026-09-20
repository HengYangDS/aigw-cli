// Package configuration owns AIGW's complete configuration domain: its
// schema, validation, persistence, and token-free interchange format.
package configuration

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
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

// Config is the typed source of truth for accounts, profiles, routes, and client adapters.
type Config struct {
	Version           int                      `json:"version"                      toml:"version"`
	Accounts          map[string]Account       `json:"accounts,omitempty"           toml:"accounts,omitempty"`
	Profiles          map[string]Profile       `json:"profiles"                     toml:"profiles"`
	Routes            Routes                   `json:"routes"                       toml:"routes"`
	RecommendedRoutes Routes                   `json:"recommended_routes,omitempty" toml:"recommended_routes,omitempty"`
	Adapters          map[string]AdapterConfig `json:"adapters,omitempty"           toml:"adapters,omitempty"`
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
	return Config{Version: ConfigVersion, Accounts: map[string]Account{}, Profiles: map[string]Profile{}, Routes: Routes{}, RecommendedRoutes: Routes{}, Adapters: map[string]AdapterConfig{}}
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
	for name, adapter := range c.Adapters {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		out.Adapters[name] = adapter
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
	clients := make([]string, 0, len(c.Adapters))
	for _, client := range AdmittedClientIDs() {
		if c.Adapters[client].Enabled {
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

// ValidIdentifier reports whether a user-defined identifier is safe for configuration and credential slots.
func ValidIdentifier(name string) bool { return profileNamePattern.MatchString(name) }

// Normalize fills deterministic defaults and canonicalizes collection state in place.
func (c *Config) Normalize() {
	if c.Accounts == nil {
		c.Accounts = map[string]Account{}
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	if c.Routes == nil {
		c.Routes = Routes{}
	}
	if c.RecommendedRoutes == nil {
		c.RecommendedRoutes = Routes{}
	}
	if c.Adapters == nil {
		c.Adapters = map[string]AdapterConfig{}
	}
}

// Validate checks schema, accounts, profiles, routes, then adapters without mutation.
// Within each collection, lexical key order determines the first diagnostic.
func (c Config) Validate() error {
	if c.Version != ConfigVersion {
		return &UnsupportedConfigVersionError{Version: c.Version, ExpectedVersion: ConfigVersion}
	}
	if len(c.Profiles) == 0 {
		return errors.New("at least one profile is required")
	}
	if len(c.Accounts) == 0 {
		return errors.New("at least one account is required")
	}
	for _, name := range slices.Sorted(maps.Keys(c.Accounts)) {
		if err := c.Accounts[name].validate(name); err != nil {
			return err
		}
	}
	for _, name := range c.ProfileIDs() {
		if err := c.Profiles[name].validate(name, c.Accounts); err != nil {
			return err
		}
	}
	if err := c.validateRoutes(c.Routes, "route"); err != nil {
		return err
	}
	if err := c.validateRoutes(c.RecommendedRoutes, "recommended route"); err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(c.Adapters)) {
		if !IsAdmittedClient(name) {
			return fmt.Errorf("unknown adapter %q", name)
		}
		command := c.Adapters[name].CredentialCommand
		if command != "" && (!filepath.IsAbs(command) || strings.TrimSpace(command) != command || strings.ContainsFunc(command, unicode.IsControl)) {
			return fmt.Errorf("adapter %q credential_command must be one absolute executable path", name)
		}
	}
	return nil
}

func (c Config) validateRoutes(routes Routes, kind string) error {
	for _, client := range slices.Sorted(maps.Keys(routes)) {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown %s %q; supported routes are %s", kind, client, AdmittedClientUsage())
		}
		profile := routes[client]
		selected, ok := c.Profiles[profile]
		if !ok {
			return fmt.Errorf("%s %q references unknown profile %q", kind, client, profile)
		}
		if selected.Client != client {
			return fmt.Errorf("%s %q selects profile %q for %q", kind, client, profile, selected.Client)
		}
	}
	return nil
}

func (account Account) validate(name string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid account name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if name != strings.ToLower(name) {
		return fmt.Errorf("invalid account name %q; environment-backed account IDs must be lowercase", name)
	}
	if strings.TrimSpace(account.Label) == "" {
		return fmt.Errorf("account %q has an empty label", name)
	}
	if account.Endpoints.OpenAIResponses == "" && account.Endpoints.Anthropic == "" && account.Endpoints.OpenAIChatCompletions == "" {
		return fmt.Errorf("account %q must define at least one endpoint", name)
	}
	for _, endpoint := range []struct {
		protocol EndpointProtocol
		url      string
	}{
		{ProtocolAnthropic, account.Endpoints.Anthropic},
		{ProtocolOpenAIResponses, account.Endpoints.OpenAIResponses},
		{ProtocolOpenAIChatCompletions, account.Endpoints.OpenAIChatCompletions},
	} {
		if endpoint.url == "" {
			continue
		}
		if err := validateEndpoint(endpoint.url); err != nil {
			return fmt.Errorf("account %q endpoint %s: %w", name, endpoint.protocol, err)
		}
	}
	if account.AccountProbe == nil {
		return nil
	}
	if !ValidIdentifier(account.AccountProbe.Kind) {
		return fmt.Errorf("account %q has invalid account probe provider %q", name, account.AccountProbe.Kind)
	}
	if err := validateEndpoint(account.AccountProbe.BaseURL); err != nil {
		return fmt.Errorf("account %q account probe: %w", name, err)
	}
	return nil
}

func (profile Profile) validate(name string, accounts map[string]Account) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid profile name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if strings.TrimSpace(profile.Label) == "" {
		return fmt.Errorf("profile %q has an empty label", name)
	}
	if profile.Account == "" {
		return fmt.Errorf("profile %q must reference an account", name)
	}
	account, ok := accounts[profile.Account]
	if !ok {
		return fmt.Errorf("profile %q references unknown account %q", name, profile.Account)
	}
	if !IsAdmittedClient(profile.Client) {
		return fmt.Errorf("profile %q has unknown client %q", name, profile.Client)
	}
	account.ID = profile.Account
	spec, _ := ClientSpecFor(profile.Client)
	if _, _, err := spec.ResolveEndpoint(account, profile.Protocol); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	if strings.TrimSpace(profile.Model) == "" {
		return fmt.Errorf("profile %q must define a model", name)
	}
	if profile.ModelProvider != "" && !modelProviderPattern.MatchString(profile.ModelProvider) {
		return fmt.Errorf("profile %q has invalid model provider %q", name, profile.ModelProvider)
	}
	if profile.ModelProvider != "" && profile.Client != ClientCodex {
		return fmt.Errorf("profile %q model_provider is only supported for codex-scoped profiles", name)
	}
	switch authentication := resolvedAuthentication(profile); authentication {
	case AuthenticationAccountToken:
	case AuthenticationClientNative:
		if profile.Client != ClientCodex {
			return fmt.Errorf("profile %q client-native authentication is only supported for codex-scoped profiles", name)
		}
		if profile.ModelProvider == "" {
			return fmt.Errorf("profile %q client-native authentication requires model_provider", name)
		}
	default:
		return fmt.Errorf("profile %q has invalid authentication %q", name, authentication)
	}
	return nil
}

func resolvedModelProvider(client string, profile Profile) string {
	if client != ClientCodex {
		return ""
	}
	if profile.ModelProvider != "" {
		return profile.ModelProvider
	}
	return ModelProviderAIGW
}

func resolvedAuthentication(profile Profile) Authentication {
	if profile.Authentication == "" {
		return AuthenticationAccountToken
	}
	return profile.Authentication
}

func validateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errors.New("URL must use http or https and include a host")
	}
	if u.User != nil {
		return errors.New("URL userinfo is forbidden")
	}
	if u.Scheme == "http" && !IsLoopbackHost(u.Hostname()) {
		return errors.New("plain HTTP is allowed only for a loopback endpoint")
	}
	for _, key := range slices.Sorted(maps.Keys(u.Query())) {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key") || strings.Contains(lower, "auth") || strings.Contains(lower, "password") {
			return fmt.Errorf("credential-like query parameter %q is forbidden", key)
		}
	}
	return nil
}

// IsLoopbackHost classifies a URL hostname without DNS or service discovery.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

// ResolveRuntime resolves one client route to its validated account and profile runtime.
func (c Config) ResolveRuntime(client, explicitProfile string) (Runtime, error) {
	name := explicitProfile
	if name == "" {
		name = c.Routes[client]
		if name == "" {
			return Runtime{}, &RuntimeRouteUnselectedError{Client: client}
		}
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown profile %q", name)
	}
	profile.ID = name
	if profile.Client != client {
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
	endpoint, protocol, err := spec.ResolveEndpoint(account, profile.Protocol)
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
		ModelProvider:     resolvedModelProvider(client, profile),
		Authentication:    resolvedAuthentication(profile),
		CredentialCommand: c.Adapters[client].CredentialCommand,
	}, nil
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
