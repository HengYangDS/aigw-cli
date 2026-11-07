// Package use implements `aigw use <client> <provider/model>`, which points
// one client at a model.
package use

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

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw use`'s JSON document.
const schemaVersion = 3

// options are the flags of `aigw use`.
type options struct {
	protocol domain.EndpointProtocol
	unlisted bool
	dryRun   bool
	jsonMode bool
}

// NewCommand constructs the command that points a client at a model.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var options options
	admission := runtime.Clients.Admission()
	cmd := &cobra.Command{
		Use:   "use <client> <provider/model>",
		Short: "Point a client at a model",
		Long: "Point a client at a model, named provider/model; <client> is " + admission.Usage() + ".\n" +
			"A model that is not added yet is added first, once the provider's catalog lists it; --unlisted skips that check.\n" +
			"In a terminal, a missing client or model is asked for, recommended models first.",
		Example: examples(admission),
		Args: func(_ *cobra.Command, args []string) error {
			given, err := parseArguments(admission, args)
			if err != nil || given.unknown {
				return err
			}
			return requireArguments(given, runtime.Interactive, options.jsonMode)
		},
		ValidArgsFunction: invocation.CompleteClientModels(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, runtime, args, options)
		},
	}
	invocation.BindProtocol(cmd, &options.protocol, "The protocol on which the client uses the model, for a client that speaks several")
	cmd.Flags().BoolVar(&options.unlisted, "unlisted", false, "Add a model that the provider's catalog does not list, without reading the catalog")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the selection without reading a Token or a catalog, sending a request or writing anything")
	cmd.Flags().BoolVar(&options.jsonMode, "json", false, "Write the selection as JSON")
	return cmd
}

// examples shows a selection for a client that speaks one protocol, one that
// names the protocol for a client that speaks several, and a preview, each
// with a client of the admission.
func examples(admission domain.Admission) string {
	var lines []string
	for _, example := range []struct {
		fits func(domain.ClientSpec) bool
		line string
	}{
		{func(spec domain.ClientSpec) bool {
			return slices.Equal(spec.EndpointProtocols, []domain.EndpointProtocol{domain.ProtocolOpenAIResponses})
		}, " dmxapi/gpt-6.1-sol"},
		{func(spec domain.ClientSpec) bool { return len(spec.EndpointProtocols) > 1 }, " ucloud/gpt-6.1-sol --protocol responses"},
		{func(spec domain.ClientSpec) bool {
			return slices.Equal(spec.EndpointProtocols, []domain.EndpointProtocol{domain.ProtocolAnthropic})
		}, " ucloud/minimax-m3 --dry-run"},
	} {
		if index := slices.IndexFunc(admission.Specs(), example.fits); index >= 0 {
			lines = append(lines, "aigw use "+admission.Specs()[index].ID+example.line)
		}
	}
	return strings.Join(lines, "\n")
}

// arguments are the client and the model that the command line names. One
// argument is the model when it has a slash, as every model name has, and the
// client otherwise; unknown marks a lone argument that names no client,
// which may name a model.
type arguments struct {
	client, model string
	unknown       bool
}

// parseArguments checks the arguments' number and shapes before any
// configuration access: a client must be admitted, and a model must be a
// provider/model name; one without a slash is left to the command, which
// lists the models that it could mean. A lone argument that names no client
// is left to the command too, which lists the models that it could mean
// before it refuses it as a client.
func parseArguments(admission domain.Admission, args []string) (arguments, error) {
	var given arguments
	switch len(args) {
	case 0:
	case 1:
		if strings.Contains(args[0], "/") {
			given.model = args[0]
		} else {
			given.client = args[0]
		}
	case 2:
		given.client, given.model = args[0], args[1]
	default:
		return arguments{}, fmt.Errorf("aigw use takes <client> and <provider/model>, not %d arguments; run `aigw use --help`", len(args))
	}
	if given.client != "" {
		if _, err := invocation.ResolveClient(admission, given.client); err != nil {
			if len(args) > 1 {
				return arguments{}, err
			}
			given.unknown = true
		}
	}
	if given.model != "" {
		if _, err := domain.ParseModelRef(given.model); err != nil && !errors.Is(err, domain.ErrModelRefWithoutProvider) {
			return arguments{}, fmt.Errorf("%w; run `aigw use --help`", err)
		}
	}
	return given, nil
}

// requireArguments refuses a missing argument where no prompt may ask for
// it: without a terminal, and with --json, whose caller reads one document.
func requireArguments(given arguments, interactive, jsonMode bool) error {
	if interactive && !jsonMode {
		return nil
	}
	why := "without a terminal"
	if jsonMode {
		why = "with --json"
	}
	switch {
	case given.client == "" && given.model == "":
		return fmt.Errorf("aigw use %s needs a <client> and a <provider/model> argument; run `aigw use --help`", why)
	case given.client == "":
		return fmt.Errorf("aigw use %s needs a <client> argument before the model; run `aigw use --help`", why)
	case given.model == "":
		return fmt.Errorf("aigw use %s needs a <provider/model> argument after the client; run `aigw use --help`", why)
	}
	return nil
}

// run resolves the selection, previews it, and unless --dry-run asks for it,
// obtains a missing Token, checks that the provider's catalog lists a model
// that the selection adds or gives a protocol, and commits the selection
// with every client file that it changes.
func run(cmd *cobra.Command, runtime invocation.Context, args []string, options options) error {
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	// Args checked the arguments, so they parse.
	given, _ := parseArguments(runtime.Clients.Admission(), args)
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if given.unknown {
		_, err := invocation.ResolveClientOrModel(cfg, runtime.Clients.Admission(), given.client)
		return err
	}
	operation := runtime
	operation.Interactive = runtime.Interactive && !options.jsonMode
	operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
	chosen, err := resolve(operation, cfg, given, options.protocol)
	if err != nil {
		return cancelledPrompt(cmd, args, err)
	}
	chosen.unlisted = options.unlisted && (chosen.adds || chosen.gains)
	synchronizer := operation.Synchronizer()
	after, selected, err := synchronizer.PrepareSelection(withAddition(cfg, chosen), chosen.client.ID, chosen.key, chosen.protocol)
	if err != nil {
		return err
	}
	missing, err := tokenMissing(operation, selected)
	if err != nil {
		return err
	}
	var presumed []string
	if missing {
		presumed = append(presumed, selected.ProviderID)
	}
	desired, projection, err := synchronizer.PreviewSelection(cfg, after, chosen.client.ID, presumed...)
	if err != nil {
		return err
	}
	outcome := selectionOutcome{
		before: cfg, after: desired, selected: selected, choice: chosen, preview: invocation.Planned(operation, projection),
		tokenMissing: missing, readOnly: secret.IsReadOnly(operation.Secrets), unlisted: options.unlisted,
	}
	if options.dryRun {
		// Only a preview names the credential reader; a commit prepares it.
		if outcome.preview, err = invocation.WithEntrypoint(operation, desired, projection); err != nil {
			return err
		}
		outcome.changed = !reflect.DeepEqual(cfg, desired)
		return outcome.report(runtime, options)
	}
	token := ""
	if missing {
		if token, err = acquireToken(cmd, operation, cfg, chosen, selected, options.jsonMode); err != nil {
			return cancelledPrompt(cmd, args, err)
		}
	}
	if (chosen.adds || chosen.gains) && !options.unlisted {
		if err := checkCatalog(cmd, operation, cfg, chosen, token); err != nil {
			if token != "" {
				// The Token is stored only with the selection, as a Token
				// that the provider rejects is not stored.
				err = invocation.WithImpact(err, "No Token was stored.")
			}
			return err
		}
		outcome.catalogChecked = true
	}
	changed, _, _, err := synchronizer.SelectModel(cmd.Context(), cfg, after, chosen.client.ID, token)
	if err != nil {
		return tokenNotKept(operation, selected.ProviderID, token, err)
	}
	outcome.changed, outcome.tokenStored = changed, token != ""
	return outcome.report(runtime, options)
}

// withAddition returns cfg with the model that the selection adds, or with
// the protocol that it gives an added model, declaring no capability on it.
// A model that the selection adds takes the family that the model ID names,
// ignoring case, or is its own family.
func withAddition(cfg domain.Config, chosen choice) domain.Config {
	if !chosen.adds && !chosen.gains {
		return cfg
	}
	working := cfg.Clone()
	model := working.Models[chosen.key]
	if chosen.adds {
		model = domain.Model{Provider: chosen.ref.Provider(), UpstreamModel: chosen.ref.ID(), Family: working.AddedModelFamily(chosen.ref.ID())}
	}
	protocols := maps.Clone(model.Protocols)
	if protocols == nil {
		protocols = map[domain.EndpointProtocol][]domain.Capability{}
	}
	protocols[chosen.protocol] = []domain.Capability{}
	model.Protocols = protocols
	working.Models[chosen.key] = model
	return working
}

// checkCatalog applies the catalog check that `aigw model add` applies, with
// the Token that the selection obtained when the store held none. Its fixes
// rerun the selection, with another model ID or with --unlisted, or select a
// model that is already added.
func checkCatalog(cmd *cobra.Command, runtime invocation.Context, cfg domain.Config, chosen choice, token string) error {
	reader := runtime
	if token != "" {
		reader.Secrets = obtainedToken{Store: runtime.Secrets, provider: chosen.ref.Provider(), token: token}
	}
	return invocation.CheckCatalog(cmd.Context(), reader, cfg, chosen.ref, invocation.CatalogCommands{
		Rerun: func(model string, added ...string) string {
			return invocation.Rerun(cmd, []string{chosen.client.ID, model}, nil, added...)
		},
		Added: func(model string) string { return invocation.Command("use", chosen.client.ID, model) },
	})
}
