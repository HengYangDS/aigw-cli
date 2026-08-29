package onboarding

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	domainverification "aigw-cli/internal/verification"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

type manifestSetupCredential struct {
	account     string
	token       string
	previous    string
	hadPrevious bool
	write       bool
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
	if len(cfg.Profiles) > 0 {
		return fmt.Errorf("AIGW is already configured; run `aigw config import %s` to merge a reviewed configuration manifest", request.From)
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
	discoveredClaude := discovered.Executable(configuration.ClientClaude)
	discoveredCodex := discovered.Executable(configuration.ClientCodex)
	discoveredTargets := discovered.AutoManagedCodexTargets()
	for _, accountName := range accountNames {
		if len(configuredClientsForAccount(cfg, accountName)) == 0 {
			return fmt.Errorf("Account %q is not referenced by any configuration profile; remove it or add an explicit client profile before setup", accountName)
		}
	}
	if request.Account != "" {
		preflight, selectErr := cfg.SelectRoutesForConnectedAccounts([]string{request.Account})
		if selectErr != nil {
			return selectErr
		}
		if _, _, resolveErr := preflight.ResolveRuntime(configuration.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 1 {
			return fmt.Errorf("configuration setup found multiple auto-managed Codex targets; automatic native credential binding is not atomic across targets, so reduce the admitted target set before setup or import the manifest without first-time client binding")
		}
	}
	credentials, err := collectManifestSetupCredentials(runtime, cfg, accountNames, request.Account, request.TokenStdin)
	if err != nil {
		return err
	}
	connectedAccounts := make([]string, 0, len(credentials))
	for _, item := range credentials {
		connectedAccounts = append(connectedAccounts, item.account)
	}
	cfg, err = cfg.SelectRoutesForConnectedAccounts(connectedAccounts)
	if err != nil {
		return err
	}
	connected := make(map[string]manifestSetupCredential, len(credentials))
	for _, item := range credentials {
		connected[item.account] = item
	}
	selectedClients := manifestSetupSelectedClients(cfg, connected, map[string]bool{
		configuration.ClientClaude: discoveredClaude != "",
		configuration.ClientCodex:  discoveredCodex != "" && len(discoveredTargets) > 0,
	})
	if containsClient(selectedClients, configuration.ClientCodex) && len(discoveredTargets) > 1 {
		return fmt.Errorf("configuration setup found multiple auto-managed Codex targets; automatic native credential binding is not atomic across targets, so reduce the admitted target set before setup or import the manifest without first-time client binding")
	}
	for _, credential := range credentials {
		if err := verifyManifestSetupCredential(ctx, runtime, cfg, credential.account, credential.token, discoveredClaude, selectedClients...); err != nil {
			return fmt.Errorf("Token validation failed for Account %q: %w", credential.account, err)
		}
	}

	if containsClient(selectedClients, configuration.ClientClaude) {
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if containsClient(selectedClients, configuration.ClientCodex) {
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: discoveredCodex, Targets: discoveredTargets}
	}

	written, err := writeManifestSetupCredentials(runtime, credentials)
	if err != nil {
		return err
	}
	if err := invocation.Synchronizer(runtime).Commit(ctx, before, cfg, "configuration setup"); err != nil {
		if rollbackErr := rollbackManifestSetupCredentials(runtime, credentials, written); rollbackErr != nil {
			return fmt.Errorf("configuration setup failed: %w; credential rollback also failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("Configuration setup failed and credentials were rolled back: %w", err)
	}

	r := invocation.Renderer(runtime)
	r.ProductTitle("Configuration catalogue imported")
	r.Section("Team catalogue")
	r.Row("Accounts", fmt.Sprintf("%d", len(accountNames)))
	r.Row("Model profiles", fmt.Sprintf("%d", len(manifest.Profiles)))
	r.Row("Connected accounts", fmt.Sprintf("%d of %d", len(credentials), len(accountNames)))
	r.Section("Accounts")
	for _, name := range accountNames {
		if _, ok := connected[name]; ok {
			r.Status(presentation.OK, name, "Connected")
		} else {
			r.Status(presentation.Info, name, "Not connected")
		}
	}
	r.Section("Clients")
	for _, spec := range configuration.AdmittedClientSpecs() {
		switch {
		case cfg.Adapters[spec.ID].Enabled:
			r.Status(presentation.OK, spec.Label, "Configured")
		case discovered.Executable(spec.ID) == "":
			r.Status(presentation.Info, spec.Label, "Not installed")
		default:
			r.Status(presentation.Info, spec.Label, "Installed · connect a compatible Account to configure")
		}
	}
	r.Success("Reviewed Accounts and Profiles are available; Tokens remain outside configuration")
	switch {
	case len(credentials) == 0:
		if secrets.IsReadOnly(runtime.Secrets) {
			for _, account := range accountNames {
				instruction, _ := credential.TokenRecovery(runtime.Secrets, account)
				r.Detail(instruction)
			}
			r.Next("aigw use " + manifest.RecommendedDefault)
		} else {
			r.Next("aigw rotate <account>")
		}
	case len(selectedClients) == 0:
		r.Detail("After installing Claude Code or Codex, run `aigw sync`")
		r.Next("aigw status")
	default:
		r.Next("aigw check")
	}
	return nil
}

func collectManifestSetupCredentials(runtime invocation.Context, cfg configuration.Config, accountNames []string, selectedAccount string, tokenStdin bool) ([]manifestSetupCredential, error) {
	if tokenStdin && selectedAccount == "" {
		return nil, fmt.Errorf("--token-stdin requires --account so one Token has one unambiguous owner")
	}
	if selectedAccount != "" {
		if _, ok := cfg.Accounts[selectedAccount]; !ok {
			return nil, fmt.Errorf("unknown Account %q; choose one of %s", selectedAccount, strings.Join(accountNames, ", "))
		}
		accountNames = []string{selectedAccount}
	}
	credentials := make([]manifestSetupCredential, 0, len(accountNames))
	for _, name := range accountNames {
		credential := manifestSetupCredential{account: name}
		previous, err := runtime.Secrets.Get(name)
		switch {
		case err == nil:
			credential.token = previous
			credential.previous = previous
			credential.hadPrevious = true
			credentials = append(credentials, credential)
		case errors.Is(err, secrets.ErrNotFound):
			continue
		default:
			return nil, err
		}
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
			credentials = append(credentials, manifestSetupCredential{account: selectedAccount})
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
	return []manifestSetupCredential{{account: selectedAccount, token: token, write: true}}, nil
}

func writeManifestSetupCredentials(runtime invocation.Context, credentials []manifestSetupCredential) ([]int, error) {
	written := make([]int, 0, len(credentials))
	for index, credential := range credentials {
		if !credential.write {
			continue
		}
		if err := runtime.Secrets.Set(credential.account, credential.token); err != nil {
			if rollbackErr := rollbackManifestSetupCredentials(runtime, credentials, written); rollbackErr != nil {
				return nil, fmt.Errorf("store Token for Account %q: %w; credential rollback also failed: %v", credential.account, err, rollbackErr)
			}
			return nil, fmt.Errorf("store Token for Account %q: %w", credential.account, err)
		}
		written = append(written, index)
	}
	return written, nil
}

func rollbackManifestSetupCredentials(runtime invocation.Context, credentials []manifestSetupCredential, written []int) error {
	var rollbackErr error
	for position := len(written) - 1; position >= 0; position-- {
		credential := credentials[written[position]]
		var err error
		if credential.hadPrevious {
			err = runtime.Secrets.Set(credential.account, credential.previous)
		} else {
			err = runtime.Secrets.Delete(credential.account)
		}
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore Token for Account %q: %w", credential.account, err))
		}
	}
	return rollbackErr
}

func configuredClientsForAccount(cfg configuration.Config, accountName string) []string {
	seen := map[string]bool{}
	genericProfile := false
	for profileName, profile := range cfg.Profiles {
		owner := profile.Account
		if owner == "" {
			owner = profileName
		}
		if owner != accountName {
			continue
		}
		specificClient := false
		if configuration.IsAdmittedClient(profile.Client) {
			seen[profile.Client] = true
			specificClient = true
		}
		for client, model := range profile.Models {
			if configuration.IsAdmittedClient(client) && strings.TrimSpace(model) != "" {
				seen[client] = true
				specificClient = true
			}
		}
		genericProfile = genericProfile || !specificClient
	}
	if genericProfile {
		account := cfg.Accounts[accountName]
		if account.Endpoints.Anthropic != "" {
			seen[configuration.ClientClaude] = true
		}
		if account.Endpoints.OpenAIResponses != "" {
			seen[configuration.ClientCodex] = true
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

func verifyManifestSetupCredential(ctx context.Context, runtime invocation.Context, cfg configuration.Config, accountName, token, claudeExecutable string, selectedClients ...string) error {
	account := cfg.Accounts[accountName]
	account.ID = accountName
	for _, client := range selectedClients {
		clientRuntime, _, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil || clientRuntime.AccountID != accountName {
			continue
		}
		if client == configuration.ClientClaude && claudeExecutable != "" {
			if err := domainverification.VerifyClaudeRuntime(ctx, runtime.Runner, claudeExecutable, clientRuntime, token); err != nil {
				return err
			}
			continue
		}
		if err := credential.Validate(ctx, runtime.HTTP, account, token, client); err != nil {
			return err
		}
	}
	return nil
}

func manifestSetupSelectedClients(cfg configuration.Config, connected map[string]manifestSetupCredential, available map[string]bool) []string {
	clients := make([]string, 0, len(configuration.AdmittedClientIDs()))
	for _, client := range configuration.AdmittedClientIDs() {
		runtime, _, err := cfg.ResolveRuntime(client, "")
		if err != nil || runtime.AccountID == "" {
			continue
		}
		if _, ok := connected[runtime.AccountID]; !ok || !available[client] {
			continue
		}
		clients = append(clients, client)
	}
	return clients
}

func containsClient(clients []string, target string) bool {
	for _, client := range clients {
		if client == target {
			return true
		}
	}
	return false
}

func firstRuntimeForAccountClient(cfg configuration.Config, accountName, client string) (configuration.Runtime, bool) {
	for _, profileName := range cfg.ProfileIDs() {
		profile := cfg.Profiles[profileName]
		owner := profile.Account
		if owner == "" {
			owner = profileName
		}
		if owner != accountName {
			continue
		}
		runtime, _, err := cfg.ResolveRuntime(client, profileName)
		if err == nil && runtime.Model != "" {
			return runtime, true
		}
	}
	return configuration.Runtime{}, false
}
