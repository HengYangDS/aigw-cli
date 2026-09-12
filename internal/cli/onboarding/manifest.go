package onboarding

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

type manifestSetupImported struct {
	Accounts []string `json:"accounts"`
	Profiles []string `json:"profiles"`
}

type manifestSetupResult struct {
	Imported          manifestSetupImported `json:"imported"`
	ConnectedAccounts []string              `json:"connected_accounts"`
	SelectedRoutes    map[string]string     `json:"selected_routes"`
	ProjectedClients  []string              `json:"projected_clients"`
	DeferredActions   []string              `json:"deferred_actions,omitempty"`
	NextAction        string                `json:"next_action"`
}

func runManifestSetup(ctx context.Context, runtime invocation.Context, request Request) error {
	data, err := os.ReadFile(request.From)
	if err != nil {
		return fmt.Errorf("Failed to read configuration manifest: %w", err)
	}
	manifest, err := configuration.Parse(data)
	if err != nil {
		return err
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if err := invocation.Synchronizer(runtime).AdmitSetup(cfg); err != nil {
		return err
	}
	before := cfg.Clone()
	cfg, err = configuration.Merge(cfg, manifest)
	if err != nil {
		return err
	}

	accountNames := configuration.ManifestAccountNames(manifest)
	discovered, err := invocation.Discover(runtime)
	if err != nil {
		return err
	}
	discoveredTargets := discovered.AutoManagedCodexTargets()
	for _, accountName := range accountNames {
		if len(configuredClientsForAccount(cfg, accountName)) == 0 {
			return fmt.Errorf("Account %q is not referenced by any configuration profile; remove it or add an explicit client profile before setup", accountName)
		}
	}

	credentials, err := collectManifestSetupCredentials(runtime, cfg, accountNames, request.Account, request.TokenStdin)
	if err != nil {
		return err
	}
	connectedAccounts := make([]string, 0, len(credentials))
	connected := make(map[string]setupCredential, len(credentials))
	tokens := make(map[string]string)
	for _, item := range credentials {
		connectedAccounts = append(connectedAccounts, item.account)
		connected[item.account] = item
		if item.write {
			tokens[item.account] = item.token
		}
	}
	cfg, err = cfg.SelectRoutesForConnectedAccounts(connectedAccounts)
	if err != nil {
		return err
	}
	selectedClients := manifestSetupSelectedClients(cfg, connected, map[string]bool{
		configuration.ClientClaude: discovered.Executable(configuration.ClientClaude) != "",
		configuration.ClientCodex:  discovered.Executable(configuration.ClientCodex) != "" && len(discoveredTargets) > 0,
	})

	for _, credential := range credentials {
		if err := verifyManifestSetupCredential(ctx, runtime, cfg, credential.account, credential.token, selectedClients...); err != nil {
			return fmt.Errorf("Token validation failed for Account %q: %w", credential.account, err)
		}
	}

	cfg, err = invocation.Synchronizer(runtime).Setup(ctx, before, cfg, tokens, selectedClients...)
	if err != nil {
		return err
	}

	availableClients := make(map[string]bool, len(configuration.AdmittedClientIDs()))
	for _, client := range configuration.AdmittedClientIDs() {
		availableClients[client] = discovered.Executable(client) != ""
	}
	result := buildManifestSetupResult(runtime, cfg, accountNames, connected, availableClients, selectedClients)
	if request.JSON {
		encoder := json.NewEncoder(runtime.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	renderManifestSetupResult(runtime, result)
	return nil
}

func buildManifestSetupResult(
	runtime invocation.Context,
	cfg configuration.Config,
	accountNames []string,
	connected map[string]setupCredential,
	availableClients map[string]bool,
	selectedClients []string,
) manifestSetupResult {
	result := manifestSetupResult{
		Imported: manifestSetupImported{
			Accounts: append([]string(nil), accountNames...),
			Profiles: cfg.ProfileIDs(),
		},
		ConnectedAccounts: make([]string, 0, len(connected)),
		SelectedRoutes:    make(map[string]string, len(cfg.Routes)),
	}
	for _, client := range selectedClients {
		if cfg.Adapters[client].Enabled {
			result.ProjectedClients = append(result.ProjectedClients, client)
		}
	}
	var accountVariables []string
	for _, name := range accountNames {
		if _, isConnected := connected[name]; isConnected {
			result.ConnectedAccounts = append(result.ConnectedAccounts, name)
		}
		if accountHasTokenAuthenticatedProfile(cfg, name) {
			accountVariables = append(accountVariables, secrets.EnvironmentKey(name))
		}
	}
	maps.Copy(result.SelectedRoutes, cfg.Routes)
	needsAccountToken := len(connected) == 0 && len(accountVariables) > 0
	if needsAccountToken {
		if secrets.IsReadOnly(runtime.Secrets) {
			result.DeferredActions = append(result.DeferredActions, "Set one compatible Account variable: "+strings.Join(accountVariables, " or "))
		} else {
			result.DeferredActions = append(result.DeferredActions, "Connect one compatible Account")
		}
	}
	for _, spec := range configuration.AdmittedClientSpecs() {
		if slices.Contains(result.ProjectedClients, spec.ID) {
			continue
		}
		if cfg.FirstProfileForClient(spec.ID) == "" {
			continue
		}
		if !availableClients[spec.ID] {
			result.DeferredActions = append(result.DeferredActions, "Install "+spec.Label+", then run `aigw sync`")
			continue
		}
		result.DeferredActions = append(result.DeferredActions, "Connect an Account compatible with "+spec.Label+", then run `aigw sync`")
	}
	switch {
	case len(result.DeferredActions) == 0:
		result.NextAction = "aigw check"
	case needsAccountToken && !secrets.IsReadOnly(runtime.Secrets):
		result.NextAction = "aigw rotate <account>"
	default:
		result.NextAction = "aigw sync"
	}
	return result
}

func renderManifestSetupResult(runtime invocation.Context, result manifestSetupResult) {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Configuration catalogue imported")
	r.Section("Imported capability")
	r.Row("Accounts", strings.Join(result.Imported.Accounts, ", "))
	r.Row("Profiles", strings.Join(result.Imported.Profiles, ", "))
	r.Row("Connected accounts", fmt.Sprintf("%d of %d", len(result.ConnectedAccounts), len(result.Imported.Accounts)))
	r.Section("Account connections")
	for _, account := range result.Imported.Accounts {
		if slices.Contains(result.ConnectedAccounts, account) {
			r.Status(presentation.OK, account, "Connected")
		} else {
			r.Status(presentation.Info, account, "Deferred")
		}
	}
	r.Section("Selected routes")
	for _, spec := range configuration.AdmittedClientSpecs() {
		profile, selected := result.SelectedRoutes[spec.ID]
		if !selected {
			r.Status(presentation.Info, spec.Label, "Deferred")
			continue
		}
		r.Status(presentation.OK, spec.Label, profile)
	}
	r.Section("Projected clients")
	if len(result.ProjectedClients) == 0 {
		r.Status(presentation.Info, "None", "Deferred")
	}
	for _, client := range result.ProjectedClients {
		spec, _ := configuration.ClientSpecFor(client)
		r.Status(presentation.OK, spec.Label, "Projected")
	}
	r.Success("Reviewed Accounts and Profiles are available; Tokens remain outside configuration")
	for _, detail := range result.DeferredActions {
		r.Detail(detail)
	}
	r.Next(result.NextAction)
}

func collectManifestSetupCredentials(runtime invocation.Context, cfg configuration.Config, accountNames []string, selectedAccount string, tokenStdin bool) ([]setupCredential, error) {
	if selectedAccount != "" {
		if _, ok := cfg.Accounts[selectedAccount]; !ok {
			return nil, fmt.Errorf("unknown Account %q; choose one of %s", selectedAccount, strings.Join(accountNames, ", "))
		}
		accountNames = []string{selectedAccount}
	}
	credentials := make([]setupCredential, 0, len(accountNames))
	for _, name := range accountNames {
		if !accountHasTokenAuthenticatedProfile(cfg, name) {
			continue
		}
		credential := setupCredential{account: name}
		available, err := runtime.Secrets.Exists(name)
		if err != nil {
			return nil, fmt.Errorf("observe Token for Account %q: %w", name, err)
		}
		if !available {
			continue
		}
		previous, err := runtime.Secrets.Get(name)
		if err != nil {
			return nil, fmt.Errorf("read Token for connected Account %q: %w", name, err)
		}
		credential.token = previous
		credentials = append(credentials, credential)
	}

	if tokenStdin {
		if secrets.IsReadOnly(runtime.Secrets) {
			return nil, fmt.Errorf("--token-stdin cannot replace a Token in the read-only environment secret backend")
		}
		token, err := invocation.ReadToken(runtime, true, true)
		if err != nil {
			return nil, err
		}
		if len(credentials) == 0 {
			credentials = append(credentials, setupCredential{account: selectedAccount})
		}
		credentials[0].token = token
		credentials[0].write = true
		return credentials, nil
	}
	if selectedAccount == "" || len(credentials) > 0 {
		return credentials, nil
	}
	if secrets.IsReadOnly(runtime.Secrets) {
		return nil, fmt.Errorf("environment Token %s is not set; provide it or choose another Account", secrets.EnvironmentKey(selectedAccount))
	}
	if !runtime.Interactive {
		return nil, fmt.Errorf("Account %q is not connected; run setup interactively or add --token-stdin", selectedAccount)
	}
	account := cfg.Accounts[selectedAccount]
	token, err := runtime.Prompt.Secret("Paste " + account.Label + " token: ")
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("empty Token refused for Account %q", selectedAccount)
	}
	return []setupCredential{{account: selectedAccount, token: token, write: true}}, nil
}

func configuredClientsForAccount(cfg configuration.Config, accountName string) []string {
	seen := map[string]bool{}
	for _, profile := range cfg.Profiles {
		if profile.Account != accountName {
			continue
		}
		if configuration.IsAdmittedClient(profile.Client) {
			seen[profile.Client] = true
		}
	}
	clients := make([]string, 0, len(seen))
	for _, client := range configuration.AdmittedClientIDs() {
		if seen[client] {
			clients = append(clients, client)
		}
	}
	return clients
}

func accountHasTokenAuthenticatedProfile(cfg configuration.Config, accountName string) bool {
	for _, profileName := range cfg.ProfileIDs() {
		profile := cfg.Profiles[profileName]
		if profile.Account != accountName {
			continue
		}
		runtime, err := cfg.ResolveRuntime(profile.Client, profileName)
		if err == nil && runtime.RequiresAccountToken() {
			return true
		}
	}
	return false
}

func verifyManifestSetupCredential(ctx context.Context, runtime invocation.Context, cfg configuration.Config, accountName, token string, selectedClients ...string) error {
	account := cfg.Accounts[accountName]
	account.ID = accountName
	for _, client := range selectedClients {
		clientRuntime, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil || clientRuntime.AccountID != accountName || !clientRuntime.RequiresAccountToken() {
			continue
		}
		if err := credential.Validate(ctx, runtime.HTTP, account, token, client); err != nil {
			return err
		}
	}
	return nil
}

func manifestSetupSelectedClients(cfg configuration.Config, connected map[string]setupCredential, available map[string]bool) []string {
	clients := make([]string, 0, len(configuration.AdmittedClientIDs()))
	for _, client := range configuration.AdmittedClientIDs() {
		runtime, err := cfg.ResolveRuntime(client, "")
		if err != nil || runtime.AccountID == "" {
			continue
		}
		if !available[client] {
			continue
		}
		if runtime.RequiresAccountToken() {
			if _, ok := connected[runtime.AccountID]; !ok {
				continue
			}
		}
		clients = append(clients, client)
	}
	return clients
}

func firstRuntimeForAccountClient(cfg configuration.Config, accountName, client string) (configuration.Runtime, bool) {
	for _, profileName := range cfg.ProfileIDs() {
		profile := cfg.Profiles[profileName]
		if profile.Account != accountName {
			continue
		}
		runtime, err := cfg.ResolveRuntime(client, profileName)
		if err == nil && runtime.Model != "" {
			return runtime, true
		}
	}
	return configuration.Runtime{}, false
}
