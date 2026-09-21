// Package configuration owns AIGW's complete configuration domain: its
// schema, validation, persistence, and token-free interchange format.
package configuration

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
)

// Authentication identifies which boundary owns credentials for a profile.
type Authentication string

const (
	// ConfigVersion is the only configuration schema version accepted by this build.
	ConfigVersion = 4
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

// Config is the typed source of truth for accounts, profiles, recommendations,
// and client bindings.
type Config struct {
	Version         int                        `json:"version"                   toml:"version"`
	Accounts        map[string]Account         `json:"accounts,omitempty"        toml:"accounts,omitempty"`
	Profiles        map[string]Profile         `json:"profiles"                  toml:"profiles"`
	Recommendations map[string]ClientSelection `json:"recommendations,omitempty" toml:"recommendations,omitempty"`
	Clients         map[string]ClientBinding   `json:"clients,omitempty"         toml:"clients,omitempty"`
}

// Account defines one provider capability and its protocol endpoints without containing credentials.
type Account struct {
	ID           string        `json:"id,omitempty"            toml:"-"`
	Label        string        `json:"label"                   toml:"label"`
	Endpoints    Endpoints     `json:"endpoints"               toml:"endpoints"`
	AccountProbe *AccountProbe `json:"account_probe,omitempty" toml:"account_probe,omitempty"`
}

// Profile identifies one upstream model on an Account independently of client
// selection and native configuration.
type Profile struct {
	ID      string `json:"id,omitempty"      toml:"-"`
	Label   string `json:"label"             toml:"label"`
	Purpose string `json:"purpose,omitempty" toml:"purpose,omitempty"`
	Account string `json:"account"           toml:"account"`
	Model   string `json:"model"             toml:"model"`
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

// NewConfig returns an empty configuration with all collection invariants initialized.
func NewConfig() Config {
	return Config{Version: ConfigVersion, Accounts: map[string]Account{}, Profiles: map[string]Profile{}, Recommendations: map[string]ClientSelection{}, Clients: map[string]ClientBinding{}}
}

// Clone returns an independent configuration value. Config is the semantic
// owner of this copy operation so mutation workflows do not reproduce its
// nested map and slice shape.
func (c *Config) Clone() Config {
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
	maps.Copy(out.Recommendations, c.Recommendations)
	for name, adapter := range c.Clients {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		out.Clients[name] = adapter
	}
	return out
}

// ProfileIDs returns the stable lexical order used by every CLI projection.
func (c *Config) ProfileIDs() []string {
	ids := make([]string, 0, len(c.Profiles))
	for id := range c.Profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ClientForProfile returns the sole admitted client compatible with a Profile.
// Callers must ask for an explicit client when more than one is compatible.
func (c *Config) ClientForProfile(name string) (string, error) {
	compatible, err := c.CompatibleClientIDs(name)
	if err != nil {
		return "", err
	}
	if len(compatible) != 1 {
		return "", fmt.Errorf("profile %q is compatible with %d clients; select one explicitly", name, len(compatible))
	}
	return compatible[0], nil
}

// CompatibleClientIDs returns admitted clients that can resolve one Profile
// without guessing between several compatible protocols.
func (c *Config) CompatibleClientIDs(name string) ([]string, error) {
	profile, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown profile %q", name)
	}
	account, ok := c.Accounts[profile.Account]
	if !ok {
		return nil, fmt.Errorf("profile %q references unknown account %q", name, profile.Account)
	}
	account.ID = profile.Account
	compatible := make([]string, 0, len(admittedClientSpecs))
	for _, client := range AdmittedClientIDs() {
		if len(mustClientSpec(client).CompatibleProtocols(account)) > 0 {
			compatible = append(compatible, client)
		}
	}
	return compatible, nil
}

// SelectedProfile returns the Profile explicitly selected by one client.
func (c *Config) SelectedProfile(client string) string { return c.Clients[client].Profile }

// SetSelectedProfile changes one client's selected Profile while preserving
// enabled intent and native options.
func (c *Config) SetSelectedProfile(client, profileID string) {
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	binding := c.Clients[client]
	if binding.Profile == "" {
		binding = binding.withSelection(c.recommendedSelection(client))
	}
	binding.Profile = profileID
	c.Clients[client] = binding
}

// SetClientActivation changes one client's enabled intent and native location
// while preserving its selected Profile and client-specific options.
func (c *Config) SetClientActivation(client string, enabled bool, executable string, targets []string) {
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	binding := c.Clients[client]
	binding.Enabled = enabled
	binding.Executable = executable
	binding.Targets = append([]string(nil), targets...)
	c.Clients[client] = binding
}

// RecommendedProfile returns the team recommendation for one client.
func (c *Config) RecommendedProfile(client string) string {
	return c.Recommendations[client].Profile
}

// SetRecommendedProfile changes one client recommendation while preserving
// its protocol and authentication choices.
func (c *Config) SetRecommendedProfile(client, profileID string) {
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientSelection{}
	}
	selection := c.Recommendations[client]
	selection.Profile = profileID
	c.Recommendations[client] = selection
}

// SelectedClientsForProfile returns the stable client set selecting one Profile.
func (c *Config) SelectedClientsForProfile(profileID string) []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range AdmittedClientIDs() {
		if c.Clients[client].Profile == profileID {
			clients = append(clients, client)
		}
	}
	return clients
}

// ResolveAccount accepts either an Account ID or a Profile ID and returns the
// referenced Account with its map identity populated.
func (c *Config) ResolveAccount(reference string) (string, Account, error) {
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

// FirstProfileForClient returns the first stable Profile that can resolve the
// requested client's model or endpoint without changing its binding.
func (c *Config) FirstProfileForClient(client string) string {
	for _, id := range c.ProfileIDs() {
		if _, err := c.ResolveRuntime(client, id); err == nil {
			return id
		}
	}
	return ""
}

// EnabledClientIDs returns the stable client scope explicitly enabled by this configuration.
func (c *Config) EnabledClientIDs() []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range AdmittedClientIDs() {
		if c.Clients[client].Enabled {
			clients = append(clients, client)
		}
	}
	return clients
}

// ClientUsesAccount reports whether the selected client binding uses an Account.
func (c *Config) ClientUsesAccount(client, accountID string) bool {
	runtime, err := c.ResolveRuntime(client, "")
	return err == nil && runtime.AccountID != "" && runtime.AccountID == accountID
}

// SelectedAccountIDs returns the stable set of Accounts selected by resolvable
// client bindings. Catalogue Accounts that no active binding selects are
// capabilities available for later connection, not current runtime
// dependencies.
func (c *Config) SelectedAccountIDs() []string {
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
// required by enabled Client Bindings. Client-native Profiles remain client
// credential concerns and therefore never create an AIGW Token requirement.
func (c *Config) RequiredAccountTokenIDs() []string {
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

// SelectProfilesForConnectedAccounts preserves the complete capability catalogue
// while filling unselected bindings whose authentication is currently usable. A
// client-native Profile is usable without an AIGW Account Token; an
// account-token Profile is usable only when its Account is connected. The
// usable recommendation wins, followed by a compatible Profile of the same
// model, then lexical Profile order. Existing selections are never replaced.
func (c *Config) SelectProfilesForConnectedAccounts(accountIDs []string, clients ...string) (Config, error) {
	selected := c.Clone()
	connected := make(map[string]bool, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := selected.Accounts[accountID]; !ok {
			return Config{}, fmt.Errorf("unknown account %q", accountID)
		}
		connected[accountID] = true
	}
	if len(clients) == 0 {
		clients = slices.Sorted(maps.Keys(c.Recommendations))
	}
	for _, client := range clients {
		binding := selected.clientBinding(client)
		if binding.Profile != "" {
			continue
		}
		selection := selected.profileForAvailableAuthentication(client, connected)
		if selection.Profile == "" {
			continue
		}
		binding = binding.withSelection(selection)
		binding.Enabled = true
		selected.Clients[client] = binding
	}
	return selected, nil
}

func (c *Config) profileForAvailableAuthentication(client string, connected map[string]bool) ClientSelection {
	recommendation := c.recommendedSelection(client)
	preferredModel := ""
	if recommendation.Profile != "" {
		runtime, err := c.resolveSelection(client, recommendation)
		if err != nil {
			return ClientSelection{}
		}
		if !runtime.RequiresAccountToken() || connected[runtime.AccountID] {
			return recommendation
		}
		preferredModel = runtime.Model
	}

	for _, profileID := range c.ProfileIDs() {
		selection := recommendation
		selection.Profile = profileID
		runtime, err := c.resolveSelection(client, selection)
		if err != nil || runtime.RequiresAccountToken() && !connected[runtime.AccountID] {
			continue
		}
		if preferredModel == "" || runtime.Model == preferredModel {
			return selection
		}
	}
	return ClientSelection{}
}

func (c *Config) recommendedSelection(client string) ClientSelection {
	return c.Recommendations[client]
}

// ResolveRuntime resolves one Client Binding to its validated Account and Profile runtime.
func (c *Config) ResolveRuntime(client, explicitProfile string) (Runtime, error) {
	binding := c.clientBinding(client)
	selection := binding.selection()
	if explicitProfile == "" && selection.Profile == "" {
		return Runtime{}, &RuntimeBindingUnselectedError{Client: client}
	}
	if explicitProfile == "" || explicitProfile == selection.Profile {
		return c.resolveSelection(client, selection)
	}
	selection = ClientSelection{Profile: explicitProfile}
	if binding.Profile == "" {
		selection = c.recommendedSelection(client)
		selection.Profile = explicitProfile
	}
	return c.resolveSelection(client, selection)
}
func (c *Config) clientBinding(client string) ClientBinding {
	return c.Clients[client]
}

func (c *Config) resolveSelection(client string, selection ClientSelection) (Runtime, error) {
	name := selection.Profile
	profile, ok := c.Profiles[name]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown profile %q", name)
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
	endpoint, protocol, err := spec.ResolveEndpoint(account, selection.Protocol)
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
		ModelProvider:     selectedModelProvider(client, selection),
		Authentication:    selectedAuthentication(selection),
		CredentialCommand: c.Clients[client].CredentialCommand,
	}, nil
}

func selectedModelProvider(client string, selection ClientSelection) string {
	if selection.ModelProvider != "" {
		return selection.ModelProvider
	}
	if client == ClientCodex {
		return ModelProviderAIGW
	}
	return ""
}

func selectedAuthentication(selection ClientSelection) Authentication {
	if selection.Authentication != "" {
		return selection.Authentication
	}
	return AuthenticationAccountToken
}

func mustClientSpec(client string) ClientSpec {
	spec, _ := ClientSpecFor(client)
	return spec
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
