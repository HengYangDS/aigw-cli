package provider

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// editOptions are the flags of `aigw provider edit`.
type editOptions struct {
	label             string
	endpoints         []string
	balanceTokenStdin bool
	balanceUserID     string
	noBalanceToken    bool
	dryRun            bool
	jsonMode          bool
}

func newEditCommand(runtime invocation.Context) *cobra.Command {
	var options editOptions
	cmd := &cobra.Command{
		Use:   "edit <provider>",
		Short: "Change a provider's label, endpoints or balance credentials; its Token stays",
		Long: "Change a provider's label, endpoints or balance credentials; its Token stays.\n" +
			"Replacing a protocol's endpoint clears the capabilities of the provider's models on that protocol, " +
			"which were qualified on the old endpoint, and the forwarding of each client that reaches it.",
		Example: "aigw provider edit dmxapi --label DMXAPI\n" +
			"aigw provider edit ucloud --endpoint anthropic=https://api.modelverse.cn --dry-run\n" +
			"aigw provider edit dmxapi --balance-token-stdin --balance-user-id 42",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := providerArgument(cmd, args); err != nil {
				return err
			}
			return checkEdit(cmd, options)
		},
		ValidArgsFunction: completeProvider(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(cmd, runtime, args, options)
		},
	}
	cmd.Flags().StringVar(&options.label, "label", "", "New display name")
	bindEndpoints(cmd, &options.endpoints, "The new endpoint for one protocol")
	cmd.Flags().BoolVar(&options.balanceTokenStdin, "balance-token-stdin", false,
		"Read the token of the provider's balance probe from standard input, or from a prompt at a terminal; needs --balance-user-id")
	cmd.Flags().StringVar(&options.balanceUserID, "balance-user-id", "", "The user ID that the balance token belongs to; needs --balance-token-stdin")
	cmd.Flags().BoolVar(&options.noBalanceToken, "no-balance-token", false, "Remove the provider's balance credentials")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the edit without reading a balance token or writing anything")
	cmd.Flags().BoolVar(&options.jsonMode, "json", false, "Write the edited provider as JSON")
	invocation.CompleteFlag(cmd, "label", cobra.NoFileCompletions)
	invocation.CompleteFlag(cmd, "balance-user-id", cobra.NoFileCompletions)
	return cmd
}

// checkEdit checks an edit's flags before any configuration access: at least
// one change, a label and endpoints that a provider may have, and either the
// two balance flags together or --no-balance-token alone.
func checkEdit(cmd *cobra.Command, options editOptions) error {
	path, changed := cmd.CommandPath(), cmd.Flags().Changed
	switch {
	case !slices.ContainsFunc([]string{"label", "endpoint", "balance-token-stdin", "balance-user-id", "no-balance-token"}, changed):
		return fmt.Errorf("provider edit needs --label, --endpoint, --balance-token-stdin with --balance-user-id, or --no-balance-token; run `%s --help`", path)
	case changed("label") && strings.TrimSpace(options.label) == "":
		return fmt.Errorf("--label requires a non-empty value; run `%s --help`", path)
	case options.balanceTokenStdin != changed("balance-user-id"):
		return fmt.Errorf("--balance-token-stdin and --balance-user-id go together; run `%s --help`", path)
	case changed("balance-user-id") && strings.TrimSpace(options.balanceUserID) == "":
		return fmt.Errorf("--balance-user-id requires a non-empty value; run `%s --help`", path)
	case options.noBalanceToken && options.balanceTokenStdin:
		return fmt.Errorf("--no-balance-token removes the balance credentials, so it takes neither --balance-token-stdin nor --balance-user-id; run `%s --help`", path)
	}
	_, err := parseEndpoints(cmd, options.endpoints)
	return err
}

func runEdit(cmd *cobra.Command, runtime invocation.Context, args []string, options editOptions) error {
	cfg, provider, err := loadProvider(runtime, args[0])
	if err != nil {
		return err
	}
	id := provider.ID
	if err := admitBalanceChange(runtime, provider, options); err != nil {
		return err
	}
	before := cfg.Clone()
	endpoints, _ := parseEndpoints(cmd, options.endpoints)
	cleared := editProvider(&cfg, id, options.label, endpoints)
	preview, err := invocation.PreviewChange(runtime, before, cfg)
	if err != nil {
		return err
	}
	next := "aigw provider show " + id
	switch {
	case options.dryRun:
		next = invocation.ApplyCommand(cmd, args)
	case len(endpoints) > 0:
		next = "aigw check"
	case options.balanceTokenStdin:
		next = "aigw provider balance " + id
	}
	if !options.dryRun {
		if err := applyEdit(cmd, runtime, args, options, before, cfg, provider); err != nil {
			return err
		}
	}
	document := changeDocument{
		providerItem: describe(cfg, id), ClearedCapabilities: cleared.capabilities, ClearedForwarding: cleared.forwarding,
		BalanceCredential: balanceChange(options), ClientFiles: invocation.ClientFiles(runtime, preview), CredentialReader: preview.CredentialEntrypoint,
	}
	return result{
		version: editSchemaVersion, title: "Provider edited", document: document, next: next,
		rows: editRows(runtime, id, cfg.Providers[id], cleared, document.BalanceCredential, options.dryRun), success: "Saved; the provider's Token is unchanged",
	}.write(runtime, options.dryRun, options.jsonMode)
}

// admitBalanceChange refuses a change of the balance credentials that cannot
// apply, before any input is read: the read-only environment backend reads
// them from its variables, and a provider without a balance probe has no use
// for a balance token. A team file sets a balance probe.
func admitBalanceChange(runtime invocation.Context, provider domain.Provider, options editOptions) error {
	if !options.balanceTokenStdin && !options.noBalanceToken {
		return nil
	}
	if variables := balanceVariables(runtime.Secrets, provider.ID); len(variables) > 0 {
		action, fix := "store", "Set "+variables[0]+" and "+variables[1]+" in aigw's environment."
		if options.noBalanceToken {
			action, fix = "remove", "Unset "+variables[0]+" and "+variables[1]+" in aigw's environment."
		}
		return terminal.ProblemError(fmt.Sprintf("the environment backend reads the %s balance credentials from %s and %s and cannot %s them",
			provider.ID, variables[0], variables[1], action), "", "", fix, nil)
	}
	if options.balanceTokenStdin && provider.BalanceProbe == nil {
		return terminal.ProblemError("provider "+provider.ID+" has no balance probe, so a balance token has no use",
			"A team file sets a provider's balance probe; `aigw setup --from <file>` applies it.", "", "aigw config export", nil)
	}
	return nil
}

// clearedSettings is what an edit of a provider's endpoints clears: the
// capabilities of its models on each replaced protocol, qualified on the old
// endpoint, and the forwarding of each client that reached it.
type clearedSettings struct {
	capabilities []clearedCapability
	forwarding   []string
}

// editProvider applies a label and endpoints to the provider in cfg. Replacing
// a protocol's endpoint clears what was qualified or set up for the old one;
// an endpoint for a new protocol, or the same URL again, clears nothing.
func editProvider(cfg *domain.Config, id, label string, endpoints []endpoint) clearedSettings {
	cleared := clearedSettings{capabilities: []clearedCapability{}, forwarding: []string{}}
	provider := cfg.Providers[id]
	if label = strings.TrimSpace(label); label != "" {
		provider.Label = label
	}
	for _, given := range endpoints {
		current := strings.TrimRight(provider.Endpoints.For(given.protocol), "/")
		if current == given.url {
			continue
		}
		if current != "" {
			cleared.capabilities = append(cleared.capabilities, clearCapabilities(cfg, id, given.protocol)...)
			cleared.forwarding = append(cleared.forwarding, clearForwarding(cfg, id, given.protocol)...)
		}
		setEndpoint(&provider.Endpoints, given.protocol, given.url)
	}
	cfg.Providers[id] = provider
	return cleared
}

// clearCapabilities clears the capabilities of the provider's models on
// protocol and returns each model whose capabilities it cleared.
func clearCapabilities(cfg *domain.Config, id string, protocol domain.EndpointProtocol) []clearedCapability {
	cleared := []clearedCapability{}
	for _, key := range cfg.CatalogModelIDs() {
		model := cfg.Models[key]
		capabilities := model.Protocols[protocol]
		if model.Provider != id || len(capabilities) == 0 {
			continue
		}
		names := make([]string, 0, len(capabilities))
		for _, capability := range capabilities {
			names = append(names, string(capability))
		}
		model.Protocols = maps.Clone(model.Protocols)
		model.Protocols[protocol] = []domain.Capability{}
		cfg.Models[key] = model
		cleared = append(cleared, clearedCapability{Model: model.Ref().String(), Protocol: protocol.Name(), Capabilities: names})
	}
	return cleared
}

// clearForwarding drops the forwarding of each client that reaches the
// provider's endpoint for protocol, and returns those clients in admission
// order. Its retry budgets stay, because they belong to the provider.
func clearForwarding(cfg *domain.Config, id string, protocol domain.EndpointProtocol) []string {
	cleared := []string{}
	for _, client := range cfg.Admission().IDs() {
		if cfg.Clients[client].ForwardingEndpoint == "" {
			continue
		}
		if runtime, err := cfg.ResolveRuntime(client, ""); err != nil || runtime.ProviderID != id || runtime.Protocol != protocol {
			continue
		}
		// Clearing forwarding sets no endpoint, which cannot fail.
		_ = cfg.SetForwardingEndpoint(client, "")
		cleared = append(cleared, client)
	}
	return cleared
}

// applyEdit writes an edit. The balance token is read first, so that input
// that is refused writes nothing; then the configuration is saved, and then
// the balance credentials change. When only they fail, the same command
// applies them again, because the saved changes repeat as no change.
func applyEdit(cmd *cobra.Command, runtime invocation.Context, args []string, options editOptions, before, after domain.Config, provider domain.Provider) error {
	var credential secret.DiagnosticCredential
	if options.balanceTokenStdin {
		token, err := readToken(runtime, "Paste the "+provider.Label+" balance token: ", !runtime.Interactive || options.jsonMode)
		if err != nil {
			return err
		}
		credential = secret.DiagnosticCredential{SystemToken: token, UserID: strings.TrimSpace(options.balanceUserID)}
	}
	if !reflect.DeepEqual(before, after) {
		if err := runtime.Synchronizer().Commit(cmd.Context(), before, after, "provider edit"); err != nil {
			return err
		}
	}
	var err error
	switch {
	case options.balanceTokenStdin:
		err = runtime.Providers.Set(provider.ID, credential)
	case options.noBalanceToken:
		if err = runtime.Providers.Delete(provider.ID); errors.Is(err, secret.ErrNotFound) {
			err = nil
		}
	}
	if err != nil {
		return terminal.ProblemError("the balance credentials could not be changed", "", "Every other change was saved.", invocation.Retry(cmd, args), err)
	}
	return nil
}

// balanceChange is what an edit does to the balance credentials: stored,
// removed, or nothing.
func balanceChange(options editOptions) string {
	switch {
	case options.balanceTokenStdin:
		return "stored"
	case options.noBalanceToken:
		return "removed"
	default:
		return ""
	}
}

// editRows are the rows of an edit's result: the provider after the edit,
// what it cleared, and what it did, or a preview would do, to the balance
// credentials.
func editRows(runtime invocation.Context, id string, provider domain.Provider, cleared clearedSettings, balance string, dryRun bool) []terminal.Field {
	rows := append(providerFields(id, provider), endpointFields(endpointItems(provider.Endpoints))...)
	for _, capability := range cleared.capabilities {
		rows = append(rows, terminal.Field{
			State: terminal.Warn, Label: "Capabilities cleared",
			Value: capability.Model + " on " + protocolLabel(capability.Protocol) + ": " + strings.Join(capability.Capabilities, ", "),
		})
	}
	if len(cleared.forwarding) > 0 {
		rows = append(rows, terminal.Field{State: terminal.Warn, Label: "Forwarding cleared", Value: clientLabels(runtime.Clients.Admission(), cleared.forwarding)})
	}
	values := map[string]string{"stored": "Stored", "removed": "Removed"}
	if dryRun {
		values = map[string]string{"stored": "Read and stored when the edit is applied", "removed": "Removed when the edit is applied"}
	}
	if value, changed := values[balance]; changed {
		rows = append(rows, terminal.Field{State: terminal.OK, Label: "Balance token", Value: value})
	}
	return rows
}
