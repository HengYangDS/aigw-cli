// Package account implements `aigw account`, which lists, edits and renames
// Accounts and manages their optional precise provider diagnostics.
package account

import (
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/provider"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// newEditCommand constructs account metadata editing. Account endpoints and
// labels belong to the account command surface, not to route management.
func newEditCommand(runtime invocation.Context) *cobra.Command {
	var label, openAIURL, anthropicURL string
	var jsonMode, dryRun bool
	cmd := &cobra.Command{
		Use: "edit <account>", Short: "Update account metadata and protocol endpoints",
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
			if !domain.ValidIdentifier(args[0]) {
				return fmt.Errorf("Invalid account ID %q; run `%s --help`", args[0], cmd.CommandPath())
			}
			for _, name := range []string{"label", "openai-url", "anthropic-url"} {
				flag := cmd.Flags().Lookup(name)
				if flag.Changed && strings.TrimSpace(flag.Value.String()) == "" {
					return fmt.Errorf("--%s requires a non-empty value; run `%s --help`", name, cmd.CommandPath())
				}
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			var preview invocation.Preview
			next := "aigw check"
			if dryRun {
				if preview, err = invocation.PreviewChange(runtime, before, cfg); err != nil {
					return err
				}
				next = invocation.ApplyCommand(cmd, args)
			} else if err := runtime.Synchronizer().Commit(cmd.Context(), before, cfg, "account edit"); err != nil {
				return err
			}
			if jsonMode {
				document := editDocument{
					DryRun: dryRun, Account: args[0], Label: providerAccount.Label, Endpoints: providerAccount.Endpoints, Preview: preview,
				}
				document.NextAction, document.NextDetail = terminal.NextSteps(next)
				return terminal.WriteDocument(runtime.Out, editSchemaVersion, true, document)
			}
			r := invocation.Renderer(runtime)
			if dryRun {
				r.ProductTitle("Account edit preview")
				r.Rows(terminal.Field{Label: "Account", Value: args[0]})
				invocation.RenderPreview(r, preview, next)
				return nil
			}
			r.ProductTitle("Account updated")
			r.Rows(terminal.Field{Label: "Account", Value: args[0]})
			r.Success("Routes using this account now use the same endpoints; token was not changed")
			r.Next(next)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the edited Account without writing it")
	cmd.Flags().StringVar(&label, "label", "", "New display name")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "New OpenAI Responses URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "New Anthropic URL")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the edited Account as JSON")
	cmd.MarkFlagsOneRequired("label", "openai-url", "anthropic-url")
	return cmd
}

// NewCommand constructs the account command group from one invocation context.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "account", Short: "Manage account endpoints and optional precise diagnostics"}
	diagnostics := &cobra.Command{Use: "diagnostics", Short: "Manage optional precise provider diagnostics"}
	var systemTokenStdin, enableJSON, disableJSON bool
	var suppliedUserID string
	enable := &cobra.Command{Use: "enable [account]", Short: "Store credentials for precise provider diagnostics", Args: cobra.MatchAll(cobra.MaximumNArgs(1), func(cmd *cobra.Command, _ []string) error {
		if systemTokenStdin {
			if strings.TrimSpace(suppliedUserID) == "" {
				return fmt.Errorf("--user-id is required with --system-token-stdin")
			}
			return nil
		}
		if enableJSON {
			return fmt.Errorf("--json reads the system token only from --system-token-stdin with --user-id; run `%s --help`", cmd.CommandPath())
		}
		if suppliedUserID != "" {
			return fmt.Errorf("--user-id requires --system-token-stdin")
		}
		if !runtime.Interactive {
			return fmt.Errorf("Enabling precise provider diagnostics requires an interactive terminal or --system-token-stdin with --user-id; run `%s --help`", cmd.CommandPath())
		}
		return nil
	}), RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		name, err := invocation.AccountReference(cfg, args)
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
		if !provider.Supports(providerAccount.AccountProbe.Kind) {
			return fmt.Errorf("This AIGW version does not include precise diagnostics for provider %q", providerAccount.AccountProbe.Kind)
		}
		var systemToken string
		if systemTokenStdin {
			systemToken, err = invocation.ReadToken(runtime, true, false)
		} else {
			systemToken, err = runtime.Prompt.Secret("Paste the platform system token (not the API token): ")
		}
		if err != nil {
			return err
		}
		userID := suppliedUserID
		if !systemTokenStdin {
			userID, err = runtime.Prompt.Text("User ID: ")
			if err != nil {
				return err
			}
		}
		if err := runtime.Accounts.Set(accountName, secret.DiagnosticCredential{SystemToken: systemToken, UserID: userID}); err != nil {
			return err
		}
		if enableJSON {
			return terminal.WriteDocument(runtime.Out, diagnosticsSchemaVersion, true, diagnosticsDocument{
				Account: accountName, Label: providerAccount.Label, Diagnostics: "enabled", NextAction: "aigw balance " + accountName,
			})
		}
		r := invocation.Renderer(runtime)
		r.ProductTitle("Account diagnostics enabled")
		r.Section("Account")
		r.Rows(
			terminal.Field{Label: "Account", Value: providerAccount.Label},
			terminal.Field{Label: "Account ID", Value: accountName},
			terminal.Field{State: terminal.OK, Label: "Diagnostic credential", Value: "Securely stored"},
		)
		r.Next("aigw balance")
		return nil
	}}
	enable.Flags().BoolVar(&systemTokenStdin, "system-token-stdin", false, "Read one platform system token from standard input")
	enable.Flags().StringVar(&suppliedUserID, "user-id", "", "Provider account user ID for explicit standard-input setup")
	enable.Flags().BoolVar(&enableJSON, "json", false, "Write the enabled diagnostics as JSON")
	disable := &cobra.Command{Use: "disable [account]", Short: "Remove credentials for precise provider diagnostics", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		name, err := invocation.AccountReference(cfg, args)
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
		if disableJSON {
			return terminal.WriteDocument(runtime.Out, diagnosticsSchemaVersion, true, diagnosticsDocument{Account: accountName, Diagnostics: "disabled"})
		}
		r := invocation.Renderer(runtime)
		r.ProductTitle("Account diagnostics disabled")
		r.Success("Provider diagnostic credentials were removed from secure storage")
		return nil
	}}
	disable.Flags().BoolVar(&disableJSON, "json", false, "Write the disabled diagnostics as JSON")
	diagnostics.AddCommand(enable, disable)
	root.AddCommand(
		newListCommand(runtime),
		newEditCommand(runtime),
		newRenameCommand(runtime),
		diagnostics,
	)
	return root
}

type accountListOutput struct {
	Accounts []domain.Account `json:"accounts"`
}

func newListCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{
		Use: "list", Short: "List account endpoints without credential values", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			result := accountListOutput{Accounts: make([]domain.Account, 0, len(cfg.Accounts))}
			for _, id := range cfg.CatalogAccountIDs() {
				account := cfg.Accounts[id]
				account.ID = id
				result.Accounts = append(result.Accounts, account)
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, listSchemaVersion, true, result)
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Accounts")
			render.Section("Configured endpoints")
			rows := make([]terminal.Field, 0, len(result.Accounts))
			for _, account := range result.Accounts {
				rows = append(rows, terminal.Field{Label: account.ID, Value: account.Label})
			}
			render.Rows(rows...)
			render.Next("aigw route list")
			return render.Err()
		},
	}
	command.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return command
}
