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
	if _, _, resolveErr := cfg.ResolveRuntime(configuration.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 1 {
		return fmt.Errorf("configuration setup found multiple auto-managed Codex targets; automatic native credential binding is not atomic across targets, so reduce the admitted target set before setup or import the manifest without first-time client binding")
	}
	for _, accountName := range accountNames {
		if len(configuredClientsForAccount(cfg, accountName)) == 0 {
			return fmt.Errorf("Account %q is not referenced by any configuration profile; remove it or add an explicit client profile before setup", accountName)
		}
	}
	credentials, err := collectManifestSetupCredentials(runtime, cfg, accountNames, request.TokenStdin)
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if err := verifyManifestSetupCredential(ctx, runtime, cfg, credential.account, credential.token, discoveredClaude); err != nil {
			return fmt.Errorf("Token validation failed for Account %q: %w", credential.account, err)
		}
	}

	if _, _, resolveErr := cfg.ResolveRuntime(configuration.ClientClaude, ""); resolveErr == nil && discoveredClaude != "" {
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if _, _, resolveErr := cfg.ResolveRuntime(configuration.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 0 {
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
	r.ProductTitle("Configuration setup")
	r.Section("Configuration manifest")
	r.Row("Accounts", fmt.Sprintf("%d", len(accountNames)))
	r.Row("Model profiles", fmt.Sprintf("%d", len(manifest.Profiles)))
	r.Row("Default profile", cfg.Routes.Default)
	r.Section("Credentials")
	for _, name := range accountNames {
		r.Status(presentation.OK, name, "Token validated")
	}
	r.Section("Clients")
	if cfg.Adapters[configuration.ClientClaude].Enabled {
		r.Status(presentation.OK, "Claude", "Configured")
	} else {
		r.Status(presentation.Info, "Claude", "Not selected")
	}
	if cfg.Adapters[configuration.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", "Configured")
	} else {
		r.Status(presentation.Info, "Codex", "Not selected")
	}
	r.Success("Configuration manifest saved; Tokens remain in system secret storage")
	r.Next("aigw check")
	return nil
}

func collectManifestSetupCredentials(runtime invocation.Context, cfg configuration.Config, accountNames []string, tokenStdin bool) ([]manifestSetupCredential, error) {
	if tokenStdin && len(accountNames) != 1 {
		return nil, fmt.Errorf("--token-stdin cannot assign one Token to a configuration manifest with multiple accounts; run setup interactively or pre-provision each Account Token")
	}
	credentials := make([]manifestSetupCredential, 0, len(accountNames))
	missing := make([]int, 0, len(accountNames))
	for _, name := range accountNames {
		credential := manifestSetupCredential{account: name}
		previous, err := runtime.Secrets.Get(name)
		switch {
		case err == nil:
			credential.token = previous
			credential.previous = previous
			credential.hadPrevious = true
		case errors.Is(err, secrets.ErrNotFound):
			missing = append(missing, len(credentials))
		default:
			return nil, err
		}
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
		credentials[0].token = token
		credentials[0].write = true
		return credentials, nil
	}
	if len(missing) == 0 {
		return credentials, nil
	}
	missingNames := make([]string, 0, len(missing))
	for _, index := range missing {
		missingNames = append(missingNames, credentials[index].account)
	}
	if secrets.IsReadOnly(runtime.Secrets) {
		return nil, fmt.Errorf("read-only environment secret backend is missing Tokens for Accounts %s; pre-provision each AIGW_TOKEN_<ACCOUNT> value", strings.Join(missingNames, ", "))
	}
	if !runtime.Interactive {
		return nil, fmt.Errorf("Accounts %s are missing Tokens; run `aigw setup --from <configuration.toml>` in an interactive terminal or pre-provision each Account Token", strings.Join(missingNames, ", "))
	}
	for _, index := range missing {
		account := cfg.Accounts[credentials[index].account]
		token, err := runtime.Prompt.Secret("Paste " + account.Label + " token: ")
		if err != nil {
			return nil, err
		}
		if token == "" {
			return nil, fmt.Errorf("empty Token refused for Account %q", credentials[index].account)
		}
		credentials[index].token = token
		credentials[index].write = true
	}
	return credentials, nil
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

func verifyManifestSetupCredential(ctx context.Context, runtime invocation.Context, cfg configuration.Config, accountName, token, claudeExecutable string) error {
	account := cfg.Accounts[accountName]
	account.ID = accountName
	for _, client := range configuredClientsForAccount(cfg, accountName) {
		if client == configuration.ClientClaude && claudeExecutable != "" {
			if clientRuntime, ok := firstRuntimeForAccountClient(cfg, accountName, client); ok {
				if err := domainverification.VerifyClaudeRuntime(ctx, runtime.Runner, claudeExecutable, clientRuntime, token); err != nil {
					return err
				}
				continue
			}
		}
		if err := credential.Validate(ctx, runtime.HTTP, account, token, client); err != nil {
			return err
		}
	}
	return nil
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
