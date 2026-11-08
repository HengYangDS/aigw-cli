// Package check implements `aigw check [client...]`, which climbs one ladder
// of evidence for each checked client, local, endpoint, model and client, as
// far as --depth asks, and says how far each client got.
package check

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// options are the flags of `aigw check`.
type options struct {
	depth      string
	model      string
	tokenStdin bool
	verbose    bool
	jsonMode   bool
}

// NewCommand constructs the command that checks whether each client works.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var opts options
	admission := runtime.Clients.Admission()
	cmd := &cobra.Command{
		Use:   "check [client...]",
		Short: "Check that each client works, as deep as --depth asks",
		Long: "Check the enabled clients, or the named ones, enabled or not; <client> is " + admission.Usage() + ".\n" +
			"Each depth runs the ones before it: local checks the configuration, the Token's presence, the\n" +
			"client files and the environment without a request; endpoint sends one authenticated request\n" +
			"that names no model; model, the default, sends one bounded inference request, which may use\n" +
			"quota; client runs a real client session, which may use more. A client stops at its first\n" +
			"failed depth, and a depth that a client cannot run is skipped with the reason.\n" +
			"Without a named client it also lists, by provider ID, the stored Tokens that no provider uses.",
		Example: examples(admission),
		Args: func(_ *cobra.Command, args []string) error {
			_, err := admit(admission, args, opts)
			return err
		},
		ValidArgsFunction: invocation.CompleteClients(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, runtime, args, opts)
		},
	}
	invocation.BindText(cmd, &opts.depth, "depth", "How far to check: local, endpoint, model or client (default model)")
	invocation.BindText(cmd, &opts.model, "model", "Try this provider/model with the one named client, without selecting it")
	cmd.Flags().BoolVar(&opts.tokenStdin, "token-stdin", false, "Read a one-time Token for --model from standard input, at the endpoint or model depth; it is neither read from nor written to the store")
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "Print every local check")
	cmd.Flags().BoolVar(&opts.jsonMode, "json", false, "Write one JSON document, which carries every local check")
	_ = cmd.RegisterFlagCompletionFunc("depth", completeDepths)
	_ = cmd.RegisterFlagCompletionFunc("model", invocation.CompleteClientModel(runtime))
	return cmd
}

// examples shows a check of the enabled clients, a session of one client that
// can run one, and every local check, with a client of the admission.
func examples(admission domain.Admission) string {
	lines := []string{"aigw check"}
	if index := slices.IndexFunc(admission.Specs(), func(spec domain.ClientSpec) bool { return !spec.InAppAcceptance }); index >= 0 {
		lines = append(lines, "aigw check "+admission.Specs()[index].ID+" --depth client")
	}
	return strings.Join(append(lines, "aigw check --depth local --verbose"), "\n")
}

// completeDepths completes --depth from the fixed list of depths.
func completeDepths(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	completions := make([]cobra.Completion, 0, len(depths))
	for _, d := range depths {
		completions = append(completions, string(d))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// admit checks the arguments before any configuration access or input: each
// named client is admitted, once, and the depth is one of the four; --model
// names a provider/model for exactly one named client, and --token-stdin
// supplies its Token at the endpoint or model depth.
func admit(admission domain.Admission, args []string, opts options) (plan, error) {
	requested, err := parseDepth(opts.depth)
	if err != nil {
		return plan{}, err
	}
	if err := admitModel(args, opts, requested); err != nil {
		return plan{}, err
	}
	for index, id := range args {
		if _, err := invocation.ResolveClient(admission, id); err != nil {
			return plan{}, err
		}
		if slices.Contains(args[:index], id) {
			return plan{}, fmt.Errorf("%s is named more than once; run `aigw check --help`", id)
		}
	}
	checked := []string{}
	for _, id := range admission.IDs() {
		if slices.Contains(args, id) {
			checked = append(checked, id)
		}
	}
	return plan{requested: requested, clients: checked, named: len(args) > 0}, nil
}

// admitModel refuses --model without exactly one named client, a malformed
// provider/model, and --token-stdin without --model or at the local or
// client depth, before standard input is read.
func admitModel(args []string, opts options, requested depth) error {
	switch {
	case opts.model == "" && opts.tokenStdin:
		return errors.New("--token-stdin needs --model, whose provider receives the Token; run `aigw check --help`")
	case opts.model == "":
		return nil
	case len(args) != 1:
		return fmt.Errorf("--model needs exactly one named client, the one that tries it, not %d; run `aigw check --help`", len(args))
	case opts.tokenStdin && requested != depthEndpoint && requested != depthModel:
		return fmt.Errorf("--token-stdin works at the endpoint or model depth, not at %s; run `aigw check --help`", requested)
	}
	if _, err := domain.ParseModelRef(opts.model); err != nil && !errors.Is(err, domain.ErrModelRefWithoutProvider) {
		return fmt.Errorf("%w; run `aigw check --help`", err)
	}
	return nil
}

// run checks the clients that the arguments name, or else the enabled ones,
// and reports them. A check that names no client while none is enabled fails
// with the step that activation names, and sends nothing. A named model is
// resolved, and the owner of a one-time Token admitted, before standard input
// is read.
func run(cmd *cobra.Command, runtime invocation.Context, args []string, opts options) error {
	// Args admitted the arguments, so they plan.
	planned, _ := admit(runtime.Clients.Admission(), args, opts)
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	operation := runtime
	operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
	if !planned.named {
		planned.clients = cfg.EnabledClientIDs()
	}
	model, err := namedModel(runtime, cfg, planned, opts)
	if err != nil {
		return err
	}
	shared, backend := sharedChecks(operation.Dependencies)
	if backend.Availability == "unavailable" {
		// The backend's recovery is this check's local depth, so the document
		// names what the shared check names instead.
		backend.RecoveryAction = backendFix
	}
	checking := checker{deps: operation, cfg: cfg, plan: planned, shared: shared, named: model}
	if len(planned.clients) == 0 {
		return noClient(operation, cfg, checking.shared, backend, planned.requested, opts.jsonMode)
	}
	result := checking.run(cmd.Context())
	if !planned.named {
		result.unused = listUnused(operation, cfg)
	}
	if result.writesSnapshot() {
		checking.saveSnapshot(cmd.Context(), &result)
	}
	if opts.jsonMode {
		doc := result.documented(backend)
		doc.Model = model.name()
		if err := terminal.WriteDocument(runtime.Out, schemaVersion, result.ok(), doc); err != nil {
			return err
		}
	} else if err := result.render(runtime, opts.verbose); err != nil {
		return err
	}
	if !result.ok() {
		// The output already said which client stopped and why.
		return terminal.Presented(result.err())
	}
	return nil
}

// namedModel resolves --model for the one named client and reads the
// one-time Token of --token-stdin, once its owner is admitted.
func namedModel(runtime invocation.Context, cfg domain.Config, planned plan, opts options) (named, error) {
	if opts.model == "" {
		return named{}, nil
	}
	model, err := resolveNamed(cfg, planned.clients[0], opts.model)
	if err != nil || !opts.tokenStdin {
		return model, err
	}
	if err := tokenOwner(cfg, planned.clients[0], model); err != nil {
		return named{}, err
	}
	model.token, err = invocation.ReadToken(runtime, true, false)
	return model, err
}

// noClient reports a check that names no client while none is enabled: no
// client is checked and nothing is sent, the activation state says what is
// missing, and its next step is the command's fix.
func noClient(runtime invocation.Context, cfg domain.Config, shared []localCheck, backend secret.BackendSelection, requested depth, jsonMode bool) error {
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	title, evidence := noClientProblem(cfg, activation)
	next := activation.NextActionFor(nil)
	if activation.State == domainreadiness.Unavailable {
		// Activation names this check's local depth, which would fail the
		// same way: the backend itself needs attention.
		next = backendFix
	}
	cause := errors.New(strings.ToLower(title[:1]) + title[1:])
	unused := listUnused(runtime, cfg)
	if !jsonMode {
		return terminal.ProblemError(title, evidence+unused.evidence(), "No client was checked.", next, cause)
	}
	doc := document{
		Depth: requested, Clients: []clientDocument{}, Checks: documentedChecks(shared), CredentialBackend: backend,
		UnusedTokens: unused, State: activation.State, Problem: title,
	}
	doc.NextAction, doc.NextDetail = terminal.NextSteps(next)
	if err := terminal.WriteDocument(runtime.Out, schemaVersion, false, doc); err != nil {
		return err
	}
	return terminal.Presented(cause)
}
