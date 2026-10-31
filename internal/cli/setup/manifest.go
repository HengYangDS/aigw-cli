package setup

import (
	"aigw-cli/internal/adapter/discovery"
	clientactivation "aigw-cli/internal/app/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// schemaVersion is the version of `aigw setup`'s JSON document, with and
// without --from.
const schemaVersion = 1

type manifestSetupImported struct {
	Accounts []string `json:"accounts"`
	Routes   []string `json:"routes"`
}

type manifestSetupResult struct {
	DryRun             bool                  `json:"dry_run,omitempty"`
	Imported           manifestSetupImported `json:"imported"`
	ConnectedAccounts  []string              `json:"connected_accounts"`
	SelectedBindings   map[string]string     `json:"selected_bindings"`
	ProjectedClients   []string              `json:"projected_clients"`
	OnlineVerification string                `json:"online_verification"`
	DeferredActions    []string              `json:"deferred_actions,omitempty"`
	// Preview is what a preview's setup would change beyond the configuration.
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

func runManifestSetup(ctx context.Context, runtime invocation.Context, request setupRequest) error {
	data, err := os.ReadFile(request.From)
	if err != nil {
		return fmt.Errorf("Failed to read configuration manifest: %w", err)
	}
	manifest, err := store.Parse(data, runtime.Config.Admission())
	if err != nil {
		return err
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if err := runtime.Synchronizer().AdmitSetup(cfg); err != nil {
		return err
	}
	before := cfg.Clone()
	cfg, err = domain.Merge(cfg, manifest)
	if err != nil {
		return err
	}

	accountNames := domain.ManifestAccountNames(manifest)
	discovered, err := runtime.Discover()
	if err != nil {
		return err
	}
	for _, accountName := range accountNames {
		if len(configuredClientsForAccount(cfg, accountName)) == 0 {
			return fmt.Errorf("Account %q is not referenced by any configuration route; remove it or add an explicit client route before setup", accountName)
		}
	}
	if request.DryRun {
		return writeManifestPreview(runtime, request, before, cfg, accountNames, discovered)
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
	availableClients := manifestSetupAvailableClients(cfg.Admission(), discovered.Executables)
	selectedClients := manifestSetupSelectedClients(cfg, connected, availableClients)

	cfg, err = runtime.Synchronizer().Setup(ctx, before, cfg, tokens, selectedClients...)
	if err != nil {
		return err
	}

	result := buildManifestSetupResult(runtime, cfg, accountNames, request.Account, connected, selectedClients)
	if request.JSON {
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, result)
	}
	renderManifestSetupResult(runtime, result)
	return nil
}

// writeManifestPreview reports what setup --from would import and the Client
// Bindings that connecting its Accounts would select. A preview reads, checks
// and stores no Token, so whether each Token is already stored is not known,
// and it connects no Account and projects no client.
func writeManifestPreview(runtime invocation.Context, request setupRequest, before, cfg domain.Config, accountNames []string, discovered discovery.Result) error {
	if request.TokenStdin && secret.IsReadOnly(runtime.Secrets) {
		return fmt.Errorf("--token-stdin cannot replace a Token in the read-only environment secret backend")
	}
	connecting := make([]string, 0, len(accountNames))
	if request.Account != "" {
		if _, ok := cfg.Accounts[request.Account]; !ok {
			return fmt.Errorf("unknown Account %q; choose one of %s", request.Account, strings.Join(accountNames, ", "))
		}
		connecting = append(connecting, request.Account)
	} else {
		for _, name := range accountNames {
			if accountHasRecommendedTokenSelection(cfg, name) {
				connecting = append(connecting, name)
			}
		}
	}
	planned, err := cfg.SelectRoutesForConnectedAccounts(connecting)
	if err != nil {
		return err
	}
	connected := make(map[string]setupCredential, len(connecting))
	for _, account := range connecting {
		connected[account] = setupCredential{account: account}
	}
	clients := manifestSetupSelectedClients(planned, connected, manifestSetupAvailableClients(planned.Admission(), discovered.Executables))
	planned, projection, err := runtime.Synchronizer().PreviewSetup(before, planned, connecting, clients...)
	if err != nil {
		return err
	}
	preview, err := invocation.WithEntrypoint(runtime, planned, projection)
	if err != nil {
		return err
	}
	result := manifestSetupResult{
		DryRun:             true,
		Imported:           manifestSetupImported{Accounts: append([]string(nil), accountNames...), Routes: planned.RouteIDs()},
		ConnectedAccounts:  []string{},
		SelectedBindings:   map[string]string{},
		ProjectedClients:   []string{},
		OnlineVerification: "not_checked",
		Preview:            preview,
	}
	for client, binding := range planned.Clients {
		if binding.Route != "" {
			result.SelectedBindings[client] = binding.Route
		}
	}
	result.NextAction, result.NextDetail = terminal.NextSteps(request.apply)
	if request.JSON {
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, result)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Configuration catalog preview")
	r.Section("Imported capability")
	r.Rows(
		terminal.Field{Label: "Accounts", Value: strings.Join(result.Imported.Accounts, ", ")},
		terminal.Field{Label: "Routes", Value: strings.Join(result.Imported.Routes, ", ")},
	)
	r.Section("Account connections")
	connections := make([]terminal.Field, 0, len(accountNames))
	for _, account := range accountNames {
		connection := terminal.Field{State: terminal.Info, Label: account, Value: "Deferred"}
		if slices.Contains(connecting, account) {
			connection.Value = "Would connect"
		}
		connections = append(connections, connection)
	}
	r.Rows(connections...)
	r.Section("Selected Client Bindings")
	bindings := make([]terminal.Field, 0, len(runtime.Clients.Admission().Specs()))
	for _, spec := range runtime.Clients.Admission().Specs() {
		binding := terminal.Field{State: terminal.Info, Label: spec.Label, Value: "Deferred"}
		if route, selected := result.SelectedBindings[spec.ID]; selected {
			binding.Value = route
		}
		bindings = append(bindings, binding)
	}
	r.Rows(bindings...)
	invocation.RenderPreview(r, preview, request.apply)
	return nil
}

func manifestSetupAvailableClients(admission domain.Admission, executables map[string]string) map[string]bool {
	available := make(map[string]bool, len(admission.IDs()))
	for _, clientID := range admission.IDs() {
		available[clientID] = executables[clientID] != ""
	}
	return available
}

func buildManifestSetupResult(
	runtime invocation.Context,
	cfg domain.Config,
	accountNames []string,
	selectedAccount string,
	connected map[string]setupCredential,
	selectedClients []string,
) manifestSetupResult {
	result := manifestSetupResult{
		Imported: manifestSetupImported{
			Accounts: append([]string(nil), accountNames...),
			Routes:   cfg.RouteIDs(),
		},
		ConnectedAccounts:  make([]string, 0, len(connected)),
		SelectedBindings:   make(map[string]string, len(cfg.Clients)),
		OnlineVerification: "not_checked",
	}
	for _, client := range selectedClients {
		if cfg.Clients[client].Enabled {
			result.ProjectedClients = append(result.ProjectedClients, client)
		}
	}
	for _, name := range accountNames {
		if _, isConnected := connected[name]; isConnected {
			result.ConnectedAccounts = append(result.ConnectedAccounts, name)
		}
	}
	for client, binding := range cfg.Clients {
		if binding.Route != "" {
			result.SelectedBindings[client] = binding.Route
		}
	}
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	explicitlyConnected := selectedAccount != "" && connected[selectedAccount].account != ""
	if activation.CredentialPrerequisite != "" && !explicitlyConnected {
		result.DeferredActions = append(result.DeferredActions, activation.CredentialPrerequisite)
	}
	for _, spec := range runtime.Clients.Admission().Specs() {
		binding, selected := cfg.Clients[spec.ID]
		if !selected || binding.Route == "" || !binding.Enabled || slices.Contains(result.ProjectedClients, spec.ID) {
			continue
		}
		if action := activation.ClientCredentialPrerequisites[spec.ID]; action != "" && !slices.Contains(result.DeferredActions, action) {
			result.DeferredActions = append(result.DeferredActions, action)
		}
		if action := activation.ProjectionPrerequisites[spec.ID]; action != "" {
			result.DeferredActions = append(result.DeferredActions, action)
		}
	}
	if explicitlyConnected {
		result.NextAction = clientactivation.NextActionAfterAccountConnection(cfg, selectedAccount)
		if result.NextAction != "aigw check" && !slices.Contains(result.DeferredActions, result.NextAction) {
			result.DeferredActions = append(result.DeferredActions, result.NextAction)
		}
	} else {
		result.NextAction = activation.NextActionFor(nil)
	}
	return result
}

func renderManifestSetupResult(runtime invocation.Context, result manifestSetupResult) {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Configuration catalog imported")
	r.Section("Imported capability")
	r.Rows(
		terminal.Field{Label: "Accounts", Value: strings.Join(result.Imported.Accounts, ", ")},
		terminal.Field{Label: "Routes", Value: strings.Join(result.Imported.Routes, ", ")},
		terminal.Field{Label: "Connected accounts", Value: fmt.Sprintf("%d of %d", len(result.ConnectedAccounts), len(result.Imported.Accounts))},
	)
	r.Section("Account connections")
	connections := make([]terminal.Field, 0, len(result.Imported.Accounts))
	for _, account := range result.Imported.Accounts {
		connection := terminal.Field{State: terminal.Info, Label: account, Value: "Deferred"}
		if slices.Contains(result.ConnectedAccounts, account) {
			connection.State, connection.Value = terminal.OK, "Connected"
		}
		connections = append(connections, connection)
	}
	r.Rows(connections...)
	r.Section("Selected Client Bindings")
	bindings := make([]terminal.Field, 0, len(runtime.Clients.Admission().Specs()))
	for _, spec := range runtime.Clients.Admission().Specs() {
		binding := terminal.Field{State: terminal.Info, Label: spec.Label, Value: "Deferred"}
		if route, selected := result.SelectedBindings[spec.ID]; selected {
			binding.State, binding.Value = terminal.OK, route
		}
		bindings = append(bindings, binding)
	}
	r.Rows(bindings...)
	r.Section("Projected clients")
	projected := make([]terminal.Field, 0, len(result.ProjectedClients)+2)
	if len(result.ProjectedClients) == 0 {
		projected = append(projected, terminal.Field{State: terminal.Info, Label: "None", Value: "Deferred"})
	}
	for _, client := range result.ProjectedClients {
		spec, _ := runtime.Clients.Admission().Spec(client)
		projected = append(projected, terminal.Field{State: terminal.OK, Label: spec.Label, Value: "Projected"})
	}
	r.Rows(append(projected, terminal.Field{State: terminal.Info, Label: "Online verification", Value: "Not checked"})...)
	r.Success("Reviewed Accounts and Routes are available; Tokens remain outside configuration")
	for _, detail := range result.DeferredActions {
		if detail != result.NextAction {
			r.Detail(detail)
		}
	}
	r.Next(result.NextAction)
}

func collectManifestSetupCredentials(runtime invocation.Context, cfg domain.Config, accountNames []string, selectedAccount string, tokenStdin bool) ([]setupCredential, error) {
	if selectedAccount != "" {
		if _, ok := cfg.Accounts[selectedAccount]; !ok {
			return nil, fmt.Errorf("unknown Account %q; choose one of %s", selectedAccount, strings.Join(accountNames, ", "))
		}
		accountNames = []string{selectedAccount}
	}
	credentials := make([]setupCredential, 0, len(accountNames))
	for _, name := range accountNames {
		if selectedAccount == "" && !accountHasRecommendedTokenSelection(cfg, name) {
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
		if secret.IsReadOnly(runtime.Secrets) {
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
	if secret.IsReadOnly(runtime.Secrets) {
		return nil, fmt.Errorf("environment Token %s is not set; provide it or choose another Account", secret.EnvironmentKey(selectedAccount))
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

func configuredClientsForAccount(cfg domain.Config, accountName string) []string {
	seen := map[string]bool{}
	for routeID, route := range cfg.Routes {
		if route.Account != accountName {
			continue
		}
		clients, err := cfg.ProtocolClientIDs(routeID)
		if err != nil {
			continue
		}
		for _, client := range clients {
			seen[client] = true
		}
	}
	clients := make([]string, 0, len(seen))
	for _, client := range cfg.Admission().IDs() {
		if seen[client] {
			clients = append(clients, client)
		}
	}
	return clients
}

func accountHasRecommendedTokenSelection(cfg domain.Config, accountName string) bool {
	for _, recommendation := range cfg.Recommendations {
		for _, selection := range recommendation.Selections() {
			route := cfg.Routes[selection.Route]
			if route.Account == accountName && selection.Authentication != domain.AuthenticationClientNative {
				return true
			}
		}
	}
	return false
}

func manifestSetupSelectedClients(cfg domain.Config, connected map[string]setupCredential, available map[string]bool) []string {
	clients := make([]string, 0, len(cfg.Admission().IDs()))
	for _, client := range cfg.Admission().IDs() {
		binding := cfg.Clients[client]
		if !binding.Enabled {
			continue
		}
		runtime, err := cfg.ResolveRuntime(client, "")
		if err != nil || runtime.AccountID == "" {
			continue
		}
		if !available[client] {
			continue
		}
		if runtime.UsesAIGWCredentialStore() {
			if _, ok := connected[runtime.AccountID]; !ok {
				continue
			}
		}
		clients = append(clients, client)
	}
	return clients
}
