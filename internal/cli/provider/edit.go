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
			if err := checkEdit(cmd, options); err != nil {
				return err
			}
			return requireBalanceInput(cmd, runtime, options)
		},
		ValidArgsFunction: completeProvider(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(cmd, runtime, args, options)
		},
	}
	invocation.BindText(cmd, &options.label, "label", "New display name")
	bindEndpoints(cmd, &options.endpoints, "The new endpoint for one protocol")
	cmd.Flags().BoolVar(&options.balanceTokenStdin, "balance-token-stdin", false,
		"Read the token of the provider's balance probe from standard input, or from a prompt at a terminal; needs --balance-user-id")
	invocation.BindText(cmd, &options.balanceUserID, "balance-user-id", "The user ID that the balance token belongs to; needs --balance-token-stdin")
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

// requireBalanceInput refuses --balance-token-stdin with --json at a
// terminal, where the balance token would come from a prompt, which --json
// never shows; piped in, the balance token is read from standard input. A
// preview reads no balance token, so it needs no input, as add's preview
// needs no Token. The fix says so in prose, because the same command at the
// same terminal fails the same way.
func requireBalanceInput(cmd *cobra.Command, runtime invocation.Context, options editOptions) error {
	if !options.balanceTokenStdin || !options.jsonMode || !runtime.Interactive || options.dryRun {
		return nil
	}
	fix := "Pipe the balance token to the same command."
	if line := invocation.Rerun(cmd, cmd.Flags().Args(), nil, "--json"); line != "" {
		fix = "Pipe the balance token to `" + line + "`."
	}
	return terminal.ProblemError("--balance-token-stdin prompts for the balance token at a terminal, and --json never prompts", "", "", fix, nil)
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
		providerItem: describe(cfg, id), ClearedCapabilities: cleared.capabilities, ClearedForwarding: cleared.forwarding, KeptRetryBudgets: cleared.budgets,
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
	// budgets are the clients that reached a replaced endpoint with retry
	// budgets, which stay, because they belong to the provider.
	budgets []string
}

// editProvider applies a label and endpoints to the provider in cfg. Replacing
// a protocol's endpoint clears what was qualified or set up for the old one;
// an endpoint for a new protocol, or the same URL again, clears nothing.
func editProvider(cfg *domain.Config, id, label string, endpoints []endpoint) clearedSettings {
	cleared := clearedSettings{capabilities: []clearedCapability{}, forwarding: []string{}, budgets: []string{}}
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
			cleared.budgets = append(cleared.budgets, budgetClients(*cfg, id, given.protocol)...)
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
	for _, ref := range cfg.ProviderModelRefs(id) {
		model := cfg.Models[ref]
		capabilities := model.Protocols[protocol]
		if len(capabilities) == 0 {
			continue
		}
		names := make([]string, 0, len(capabilities))
		for _, capability := range capabilities {
			names = append(names, string(capability))
		}
		model.Protocols = maps.Clone(model.Protocols)
		model.Protocols[protocol] = []domain.Capability{}
		cfg.Models[ref] = model
		cleared = append(cleared, clearedCapability{Model: ref.String(), Protocol: protocol.Name(), Capabilities: names})
	}
	return cleared
}

// budgetClients returns, in admission order, the clients that reach the
// provider's endpoint for protocol with retry budgets.
func budgetClients(cfg domain.Config, id string, protocol domain.EndpointProtocol) []string {
	clients := []string{}
	for _, client := range cfg.Admission().IDs() {
		if cfg.Clients[client].RetryPolicy == nil {
			continue
		}
		if runtime, err := cfg.ResolveRuntime(client, domain.ModelRef{}); err == nil && runtime.ProviderID == id && runtime.Protocol == protocol {
			clients = append(clients, client)
		}
	}
	return clients
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
		if runtime, err := cfg.ResolveRuntime(client, domain.ModelRef{}); err != nil || runtime.ProviderID != id || runtime.Protocol != protocol {
			continue
		}
		// Clearing forwarding sets no endpoint, which cannot fail.
		_ = cfg.SetForwardingEndpoint(client, "")
		cleared = append(cleared, client)
	}
	return cleared
}

// applyEdit writes an edit. The balance token is read first, so that input
// that is refused writes nothing. The balance credentials change next and the
// configuration last, so that a save that fails can undo the balance change;
// when that undo fails too, the failure names what stays changed, and the
// same command converges.
func applyEdit(cmd *cobra.Command, runtime invocation.Context, args []string, options editOptions, before, after domain.Config, provider domain.Provider) error {
	var credential secret.DiagnosticCredential
	if options.balanceTokenStdin {
		token, err := readToken(runtime, "Paste the "+provider.Label+" balance token: ", !runtime.Interactive)
		if err != nil {
			return err
		}
		credential = secret.DiagnosticCredential{SystemToken: token, UserID: strings.TrimSpace(options.balanceUserID)}
	}
	changes, err := changeBalance(runtime, provider.ID, options, credential)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(before, after) {
		return nil
	}
	if err := runtime.Synchronizer().Commit(cmd.Context(), before, after, "provider edit"); err != nil {
		return saveFailure(err, changes, "the edit of provider "+provider.ID+" was not saved", invocation.Retry(cmd, args))
	}
	return nil
}

// changeBalance stores or removes the provider's balance credentials, as the
// edit asks, and returns the change that undoes it. It reads the previous
// credentials first; ones that cannot be read are still replaced or removed,
// but nothing can put them back. A write that fails changes nothing, and its
// fix shows the provider's credentials as the backend reports them.
func changeBalance(runtime invocation.Context, id string, options editOptions, credential secret.DiagnosticCredential) ([]credentialChange, error) {
	if !options.balanceTokenStdin && !options.noBalanceToken {
		return nil, nil
	}
	previous, readErr := runtime.Providers.Get(id)
	absent := errors.Is(readErr, secret.ErrNotFound)
	if options.noBalanceToken && absent {
		return nil, nil
	}
	change := credentialChange{undone: "the balance credentials were put back", kept: "the balance credentials stay deleted"}
	if readErr == nil {
		change.undo = func() error { return runtime.Providers.Set(id, previous) }
	}
	verb := "removed"
	var err error
	if options.balanceTokenStdin {
		verb, err = "stored", runtime.Providers.Set(id, credential)
		change.undone, change.kept = "the previous balance credentials were put back", "the new balance credentials stay stored"
		if absent {
			change.undone, change.undo = "the new balance credentials were deleted again", func() error { return runtime.Providers.Delete(id) }
		}
	} else if err = runtime.Providers.Delete(id); errors.Is(err, secret.ErrNotFound) {
		err = nil
	}
	if err != nil {
		return nil, terminal.ProblemError("the balance credentials of provider "+id+" could not be "+verb, "", "Nothing changed.", "aigw provider show "+id, err)
	}
	return []credentialChange{change}, nil
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
	if len(cleared.budgets) > 0 {
		rows = append(rows, terminal.Field{State: terminal.Info, Label: "Retry budgets kept", Value: clientLabels(runtime.Clients.Admission(), cleared.budgets)})
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
