// Package account owns Account lifecycle commands, including creation,
// inspection, credential rotation, balance reporting, and removal.
package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aigw-cli/internal/account"
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
	"aigw-cli/internal/secrets"
	"github.com/spf13/cobra"
)

func NewAddCommand(runtime invocation.Context) *cobra.Command {
	var label, openAIURL, anthropicURL, client, model string
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "add <profile>",
		Short: "Add a service and its token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !configuration.ValidProfileName(name) {
				return fmt.Errorf("Invalid service ID %q; use letters, numbers, dots, hyphens, or underscores", name)
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if _, exists := cfg.Profiles[name]; exists {
				return fmt.Errorf("Profile %q already exists; use `aigw profile edit %s` or `aigw rotate %s`", name, name, name)
			}
			if label == "" {
				label = name
			}
			if !configuration.IsAdmittedClient(client) || strings.TrimSpace(model) == "" {
				return fmt.Errorf("--for and --model are required; --for must be %s", configuration.AdmittedClientUsage())
			}
			account := configuration.Account{Label: label, Endpoints: configuration.Endpoints{
				OpenAIResponses: strings.TrimRight(openAIURL, "/"),
				Anthropic:       strings.TrimRight(anthropicURL, "/"),
			}}
			account.ID = name
			if _, err := account.EndpointFor(client); err != nil {
				return err
			}
			profile := configuration.Profile{Label: label, Account: name, Client: client, Model: strings.TrimSpace(model)}
			token, err := invocation.ReadToken(runtime, tokenStdin, true)
			if err != nil {
				return err
			}
			cfg.Accounts[name] = account
			cfg.Profiles[name] = profile
			cfg.Routes[client] = name
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := runtime.Secrets.Set(name, token); err != nil {
				return err
			}
			if err := runtime.Config.Save(cfg); err != nil {
				_ = runtime.Secrets.Delete(name)
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Service added")
			r.Section("Service")
			r.Row("Name", label)
			r.Row("Configuration", name)
			r.Status(presentation.OK, "System secret", "Securely stored")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&client, "for", "", "Client: "+configuration.AdmittedClientUsage())
	cmd.Flags().StringVar(&model, "model", "", "Upstream model ID")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}

// newEditCommand constructs account metadata editing. Account endpoints and
// labels belong to the account command surface, not to profile management.
func newEditCommand(runtime invocation.Context) *cobra.Command {
	var label, openAIURL, anthropicURL string
	cmd := &cobra.Command{
		Use: "edit <account>", Short: "Update account metadata and protocol endpoints", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if label == "" && openAIURL == "" && anthropicURL == "" {
				return fmt.Errorf("Nothing to update; provide --label, --openai-url, or --anthropic-url")
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			providerAccount, exists := cfg.Accounts[args[0]]
			if !exists {
				return fmt.Errorf("Unknown account %q", args[0])
			}
			before := cfg.Clone()
			if label != "" {
				providerAccount.Label = label
			}
			if openAIURL != "" {
				providerAccount.Endpoints.OpenAIResponses = strings.TrimRight(openAIURL, "/")
			}
			if anthropicURL != "" {
				providerAccount.Endpoints.Anthropic = strings.TrimRight(anthropicURL, "/")
			}
			cfg.Accounts[args[0]] = providerAccount
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "account edit"); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Account updated")
			r.Row("Account", args[0])
			r.Success("Profiles using this account now use the same endpoints; token was not changed")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "New display name")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "New OpenAI Responses URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "New Anthropic URL")
	return cmd
}

func NewRotateCommand(runtime invocation.Context) *cobra.Command {
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "rotate [account]",
		Short: "Update the current account token",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			name, err := accountReference(cfg, args)
			if err != nil {
				return err
			}
			accountName, account, err := cfg.ResolveAccount(name)
			if err != nil {
				return err
			}
			if instruction, writable := credential.TokenRecovery(runtime.Secrets, accountName); !writable {
				cause := fmt.Errorf("credential backend is read-only")
				return invocation.Problem(
					runtime,
					"Token cannot be rotated by this credential backend",
					"The selected environment backend reads Tokens from the parent process and does not persist changes.",
					"No Token was read, validated, or changed.",
					instruction,
					cause,
				)
			}
			oldToken, oldErr := runtime.Secrets.Get(accountName)
			if oldErr != nil && !errors.Is(oldErr, secrets.ErrNotFound) {
				return oldErr
			}
			var token string
			if tokenStdin {
				token, err = invocation.ReadToken(runtime, true, false)
			} else if runtime.Interactive {
				token, err = runtime.Prompt.Secret("Paste " + account.Label + " token: ")
			} else {
				return fmt.Errorf("Token input requires an interactive terminal; pipe one token line to `aigw rotate %s --token-stdin`", accountName)
			}
			if err != nil {
				return err
			}
			account.ID = accountName
			if err := credential.Validate(context.Background(), runtime.HTTP, account, token); err != nil {
				return fmt.Errorf("Token validation failed: %w", err)
			}
			if err := runtime.Secrets.Set(accountName, token); err != nil {
				return err
			}
			synchronizer := invocation.Synchronizer(runtime)
			syncCredentials := synchronizer.UsesCredentialAccount(cfg, accountName)
			if syncCredentials {
				if err := synchronizer.Reconcile(cmd.Context(), cfg, cfg); err != nil {
					var rollbackErr error
					if errors.Is(oldErr, secrets.ErrNotFound) {
						rollbackErr = runtime.Secrets.Delete(accountName)
					} else {
						rollbackErr = runtime.Secrets.Set(accountName, oldToken)
					}
					if rollbackErr == nil {
						rollbackErr = synchronizer.Reconcile(cmd.Context(), cfg, cfg)
					}
					if rollbackErr != nil {
						return fmt.Errorf("Token synchronization failed: %w; rollback also failed: %v", err, rollbackErr)
					}
					return fmt.Errorf("Token synchronization failed and was rolled back: %w", err)
				}
				if err := synchronizer.BindCredentialsForAccount(cmd.Context(), cfg, accountName); err != nil {
					var rollbackErr error
					if errors.Is(oldErr, secrets.ErrNotFound) {
						rollbackErr = runtime.Secrets.Delete(accountName)
					} else {
						rollbackErr = runtime.Secrets.Set(accountName, oldToken)
					}
					if rollbackErr == nil {
						rollbackErr = synchronizer.Reconcile(cmd.Context(), cfg, cfg)
						if rollbackErr == nil {
							rollbackErr = synchronizer.BindCredentialsForAccount(cmd.Context(), cfg, accountName)
						}
					}
					if rollbackErr != nil {
						return fmt.Errorf("Token authentication synchronization failed: %w; rollback also failed: %v", err, rollbackErr)
					}
					return fmt.Errorf("Token authentication synchronization failed and was rolled back: %w", err)
				}
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Token updated")
			r.Section("Service")
			r.Row("Account", account.Label)
			r.Row("Account", accountName)
			r.Status(presentation.OK, "Token", "Validated and securely stored")
			if syncCredentials {
				r.Success("Client authentication synchronized")
			} else {
				r.Success("No native client credential projection uses this Account")
			}
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}
func NewCommand(runtime invocation.Context, renameCommand *cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "account", Short: "Manage account endpoints and optional precise diagnostics"}
	root.AddCommand(
		newEditCommand(runtime),
		renameCommand,
		&cobra.Command{Use: "connect [account]", Short: "Bind provider platform credentials to query precise balance", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			if !runtime.Interactive {
				return fmt.Errorf("Binding platform credentials requires an interactive terminal")
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			name, err := accountReference(cfg, args)
			if err != nil {
				return err
			}
			accountName, providerAccount, err := cfg.ResolveAccount(name)
			if err != nil {
				return err
			}
			if providerAccount.AccountProbe == nil {
				return fmt.Errorf("Account %q does not support precise account diagnostics", accountName)
			}
			if !providers.Supports(providerAccount.AccountProbe.Kind) {
				return fmt.Errorf("This AIGW version does not include precise diagnostics for provider %q", providerAccount.AccountProbe.Kind)
			}
			systemToken, err := runtime.Prompt.Secret("Paste the platform system token (not the API token): ")
			if err != nil {
				return err
			}
			userID, err := runtime.Prompt.Text("User ID: ")
			if err != nil {
				return err
			}
			if err := runtime.Accounts.Set(accountName, account.Credential{SystemToken: systemToken, UserID: userID}); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Account diagnostics enabled")
			r.Section("Service")
			r.Row("Name", providerAccount.Label)
			r.Status(presentation.OK, "System credential", "Securely stored")
			r.Next("aigw balance")
			return nil
		}},
		&cobra.Command{Use: "disconnect [account]", Short: "Remove optional provider platform credentials", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			name, err := accountReference(cfg, args)
			if err != nil {
				return err
			}
			accountName, _, err := cfg.ResolveAccount(name)
			if err != nil {
				return err
			}
			if err := runtime.Accounts.Delete(accountName); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Account diagnostics disabled")
			r.Success("Platform system credentials were removed from secure storage")
			return nil
		}},
	)
	return root
}

func NewBalanceCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "balance [account]", Short: "Show account balance and token quota", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		name, err := accountReference(cfg, args)
		if err != nil {
			return err
		}
		accountName, providerAccount, err := cfg.ResolveAccount(name)
		if err != nil {
			return err
		}
		if providerAccount.AccountProbe == nil {
			return fmt.Errorf("%s does not support precise balance queries", providerAccount.Label)
		}
		if !providers.Supports(providerAccount.AccountProbe.Kind) {
			return fmt.Errorf("Precise diagnostics provider %q is not included in this AIGW version; continue using `aigw check` for general diagnostics", providerAccount.AccountProbe.Kind)
		}
		credential, err := runtime.Accounts.Get(accountName)
		if err != nil {
			return presentation.ProblemError(
				"Precise balance diagnostics are not enabled",
				"Missing "+accountName+" provider platform query credentials; the API token is stored separately in system secret storage.",
				"Cannot distinguish account balance, remaining token quota, disabled token state, and request limits.",
				"aigw account connect "+accountName,
				err,
			)
		}
		apiToken, err := runtime.Secrets.Get(accountName)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		providerAccount.ID = accountName
		report, err := providers.Probe(ctx, runtime.HTTP, providerAccount, apiToken, credential)
		if err != nil {
			return err
		}
		status := map[string]string{"enabled": "Enabled", "disabled": "Disabled"}[report.TokenStatus]
		remaining := fmt.Sprintf("$%.4f", report.TokenRemaining)
		if report.TokenUnlimitedQuota {
			remaining = "Unlimited"
		}
		count := fmt.Sprintf("%d requests", report.TokenRemainingCount)
		if report.TokenUnlimitedCount {
			count = "Unlimited requests"
		}
		r := invocation.Renderer(runtime)
		r.ProductTitle("Account and quota")
		r.Section("Account")
		r.Row("Account", providerAccount.Label)
		r.Row("Account balance", fmt.Sprintf("$%.4f", report.AccountBalance))
		r.Section("Current API token")
		r.Row("Name", report.TokenName)
		state := presentation.OK
		if report.TokenStatus != "enabled" {
			state = presentation.Fail
		}
		r.Status(state, "Token status", status)
		r.Row("Quota used", fmt.Sprintf("$%.4f", report.TokenUsed))
		r.Row("Remaining quota", remaining)
		r.Row("Remaining requests", count)
		r.Next("aigw check")
		return nil
	}}
}

func accountReference(cfg configuration.Config, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if len(cfg.Accounts) != 1 {
		return "", fmt.Errorf("account is required when configuration contains %d accounts", len(cfg.Accounts))
	}
	for accountID := range cfg.Accounts {
		return accountID, nil
	}
	return "", fmt.Errorf("no account is configured")
}
