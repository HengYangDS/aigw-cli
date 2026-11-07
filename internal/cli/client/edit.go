package client

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// editOptions are the flags of `aigw client edit`.
type editOptions struct {
	executable string
	targets    []string
	via        string
	direct     bool
	request    budget
	stream     budget
	native     bool
	dryRun     bool
	jsonMode   bool
}

// editFlags are the flags of `aigw client edit` that change something.
var editFlags = []string{"executable", "target", "via", "direct", "request-retries", "stream-retries", "native-retries"}

func newEditCommand(runtime invocation.Context) *cobra.Command {
	options := &editOptions{}
	admission := runtime.Clients.Admission()
	cmd := &cobra.Command{
		Use:   "edit <client>",
		Short: "Change a client's executable, configuration files, forwarding or retry budgets",
		Long: "Change a client's executable, configuration files, forwarding or retry budgets; <client> is " + admission.Usage() + ".\n" +
			"Forwarding sends the client to another endpoint, such as a local proxy; its provider, model and Token stay.\n" +
			"Retry budgets belong to the client's provider, so they stay when forwarding changes.",
		Example: editExamples(admission),
		Args: func(cmd *cobra.Command, args []string) error {
			spec, err := clientArgument(cmd, admission, args)
			if err != nil {
				return err
			}
			return checkEdit(cmd, admission, spec, options)
		},
		ValidArgsFunction: completeClient(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, _ := admission.Spec(args[0])
			return runEdit(cmd, runtime, spec, *options)
		},
	}
	invocation.BindText(cmd, &options.executable, "executable", "Path to the client's executable")
	cmd.Flags().StringArrayVar(&options.targets, "target", nil, "The client's configuration `file`, for "+
		clientsWith(admission, func(spec domain.ClientSpec) bool { return spec.TargetFile != "" })+"; repeatable, and the files given replace the current ones")
	invocation.BindText(cmd, &options.via, "via", "Send the client to the endpoint `url` instead of its provider's")
	cmd.Flags().BoolVar(&options.direct, "direct", false, "Remove forwarding, so the client reaches its provider's endpoint again")
	retries := clientsWith(admission, func(spec domain.ClientSpec) bool { return spec.RetryBudgets })
	cmd.Flags().Var(&options.request, "request-retries", "The client's native request retry budget, 0 for none; "+retries)
	cmd.Flags().Var(&options.stream, "stream-retries", "The client's native stream retry budget, 0 for none; "+retries)
	cmd.Flags().BoolVar(&options.native, "native-retries", false, "Remove both retry budgets, so the client uses its own defaults; "+retries)
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the edit and the client files it changes without writing anything")
	cmd.Flags().BoolVar(&options.jsonMode, "json", false, "Write the edited client as JSON")
	invocation.CompleteFlag(cmd, "via", cobra.NoFileCompletions)
	return cmd
}

// editExamples shows edits of forwarding and of retry budgets with the first
// client that accepts both.
func editExamples(admission domain.Admission) string {
	for _, spec := range admission.Specs() {
		if spec.RetryBudgets {
			return strings.Join([]string{
				"aigw client edit " + spec.ID + " --via http://127.0.0.1:8792/v1",
				"aigw client edit " + spec.ID + " --request-retries 0 --stream-retries 0 --dry-run",
				"aigw client edit " + spec.ID + " --direct --native-retries",
			}, "\n")
		}
	}
	return ""
}

// clientsWith names, in prose, the labels of the clients that accept
// something.
func clientsWith(admission domain.Admission, accepts func(domain.ClientSpec) bool) string {
	labels := []string{}
	for _, spec := range admission.Specs() {
		if accepts(spec) {
			labels = append(labels, spec.Label)
		}
	}
	switch len(labels) {
	case 0:
		return "no client"
	case 1:
		return labels[0]
	default:
		return strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
	}
}

// budget is the value of a retry-budget flag: one count of retries. A second
// value is refused while the flags are parsed, so the last of two never wins
// silently, and a count above TOML's largest integer is refused too.
type budget struct {
	value uint64
	given bool
}

func (b *budget) Set(text string) error {
	if b.given {
		return errors.New("this flag takes one value")
	}
	value, err := strconv.ParseUint(text, 10, 63)
	if err != nil {
		return errors.New("expected a whole number of retries from 0 to 9223372036854775807")
	}
	b.value, b.given = value, true
	return nil
}

// String is the count, which a command line that rebuilds the flag writes.
func (b *budget) String() string {
	if !b.given {
		return ""
	}
	return strconv.FormatUint(b.value, 10)
}

// Type names the value in help.
func (*budget) Type() string { return "count" }

// checkEdit checks an edit's flags before any configuration access: at least
// one change, non-empty paths and URL, a URL that an endpoint may have, and
// only the flags that the client accepts. --target names configuration files
// of a client that has them, and the retry flags are for a client with native
// retry budgets.
func checkEdit(cmd *cobra.Command, admission domain.Admission, spec domain.ClientSpec, options *editOptions) error {
	path, changed := cmd.CommandPath(), cmd.Flags().Changed
	budgets := options.request.given || options.stream.given
	switch {
	case !slices.ContainsFunc(editFlags, changed):
		return fmt.Errorf("client edit needs --executable, --target, --via, --direct, --request-retries, --stream-retries or --native-retries; run `%s --help`", path)
	case changed("executable") && strings.TrimSpace(options.executable) == "":
		return fmt.Errorf("--executable requires a non-empty path; run `%s --help`", path)
	case slices.ContainsFunc(options.targets, func(target string) bool { return strings.TrimSpace(target) == "" }):
		return fmt.Errorf("--target requires a non-empty path; run `%s --help`", path)
	case len(options.targets) > 0 && spec.TargetFile == "":
		return fmt.Errorf("%s has no configuration file to name; --target is for %s; run `%s --help`",
			spec.Label, clientsWith(admission, func(spec domain.ClientSpec) bool { return spec.TargetFile != "" }), path)
	case changed("via") && options.direct:
		return fmt.Errorf("--via and --direct are mutually exclusive; run `%s --help`", path)
	case changed("via") && strings.TrimSpace(options.via) == "":
		return fmt.Errorf("--via requires an endpoint URL, and --direct removes forwarding; run `%s --help`", path)
	case options.native && budgets:
		return fmt.Errorf("--native-retries and explicit retry budgets are mutually exclusive; run `%s --help`", path)
	case !spec.RetryBudgets && (options.native || budgets):
		return fmt.Errorf("%s has no native retry budgets; --request-retries, --stream-retries and --native-retries are for %s; run `%s --help`",
			spec.Label, clientsWith(admission, func(spec domain.ClientSpec) bool { return spec.RetryBudgets }), path)
	}
	if changed("via") {
		// The refusal never repeats the URL, which may hold a credential.
		if err := domain.ValidateEndpoint(options.via); err != nil {
			return fmt.Errorf("--via: %w; run `%s --help`", err, path)
		}
	}
	_, err := absoluteTargets(cmd, options.targets)
	return err
}

// absoluteTargets returns each --target as an absolute path, so that the
// configuration names the same file from any directory, and refuses a file
// named twice.
func absoluteTargets(cmd *cobra.Command, given []string) ([]string, error) {
	targets := make([]string, 0, len(given))
	for _, target := range given {
		absolute, err := filepath.Abs(target)
		if err != nil {
			return nil, fmt.Errorf("--target %s: %w; run `%s --help`", target, err, cmd.CommandPath())
		}
		if slices.Contains(targets, absolute) {
			return nil, fmt.Errorf("--target names %s twice; run `%s --help`", absolute, cmd.CommandPath())
		}
		targets = append(targets, absolute)
	}
	return targets, nil
}

func runEdit(cmd *cobra.Command, runtime invocation.Context, spec domain.ClientSpec, options editOptions) error {
	// Args checked both paths, so neither fails here.
	targets, _ := absoluteTargets(cmd, options.targets)
	if options.executable != "" {
		options.executable, _ = filepath.Abs(options.executable)
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if err := admitConnection(cmd, cfg, spec, options); err != nil {
		return err
	}
	if err := checkTargets(cmd, runtime, spec.ID, targets); err != nil {
		return err
	}
	if err := admitTargets(runtime, cfg, spec, targets); err != nil {
		return err
	}
	before := cfg.Clone()
	if err := editClient(&cfg, spec.ID, options, targets); err != nil {
		return err
	}
	document := describeEdit(cfg, spec.ID)
	document.Changed = !reflect.DeepEqual(before, cfg)
	preview := invocation.Preview{}
	if document.Changed {
		if preview, err = invocation.PreviewProjection(runtime, before, cfg, spec.ID); err != nil {
			return targetFailure(spec.ID, targets, err)
		}
	}
	document.ClientFiles, document.CredentialReader = invocation.ClientFiles(runtime, preview), preview.CredentialEntrypoint
	document.RestartRequired = spec.RestartAfterProjection && len(document.ClientFiles) > 0
	if !options.dryRun && document.Changed {
		if err := runtime.Synchronizer().CommitProjection(cmd.Context(), before, cfg, "client edit", spec.ID); err != nil {
			return targetFailure(spec.ID, targets, err)
		}
	}
	return writeEdit(cmd, runtime, spec, document, options)
}

// targetFailure gives a failure to plan or write the client's files that
// states no fix of its own, when the edit named configuration files, the edit
// that names others: a file that it named may not exist or not be writable.
// The failure keeps what it says about the configuration's restoration.
func targetFailure(client string, targets []string, err error) error {
	if len(targets) == 0 || terminal.StatesFix(err) {
		return err
	}
	problem := terminal.ProblemFor(err)
	return terminal.ProblemError(problem.Title, problem.Evidence, problem.Impact, "aigw client edit "+client+" --target <file>", err)
}

// admitConnection refuses forwarding or retry budgets for a client that
// selects no model: both belong to its connection to the model's provider.
func admitConnection(cmd *cobra.Command, cfg domain.Config, spec domain.ClientSpec, options editOptions) error {
	if cfg.SelectedModel(spec.ID) != "" || !cmd.Flags().Changed("via") && !options.request.given && !options.stream.given {
		return nil
	}
	return terminal.ProblemError(spec.Label+" selects no model, so it has no provider connection to forward or to give retry budgets",
		"", "", "aigw use "+spec.ID+" <provider/model>", nil)
}

// checkTargets refuses a file that the client's adapter does not accept as a
// configuration file, such as an executable or another client's file.
func checkTargets(cmd *cobra.Command, runtime invocation.Context, id string, targets []string) error {
	if len(targets) == 0 || !runtime.Clients.ValidatesTargets(id) {
		return nil
	}
	discovered, err := runtime.Discover()
	if err != nil {
		return err
	}
	for _, target := range targets {
		if err := runtime.Clients.ValidateTarget(id, discovered, target); err != nil {
			return fmt.Errorf("--target %s: %w; run `%s --help`", target, err, cmd.CommandPath())
		}
	}
	return nil
}

// admitTargets refuses to change an enabled client's configuration files
// while its provider's Token is missing or cannot be observed: the transaction
// then writes no client file, and AIGW's settings would stay behind in the
// files that the edit drops, which no later synchronization visits.
func admitTargets(runtime invocation.Context, cfg domain.Config, spec domain.ClientSpec, targets []string) error {
	if len(targets) == 0 || !cfg.Clients[spec.ID].Enabled {
		return nil
	}
	selected, err := cfg.ResolveRuntime(spec.ID, "")
	if err != nil || !selected.UsesAIGWCredentialStore() {
		return nil
	}
	available, err := runtime.Secrets.Exists(selected.ProviderID)
	if err == nil && available {
		return nil
	}
	title := fmt.Sprintf("%s's configuration files cannot change while provider %s has no Token", spec.Label, selected.ProviderID)
	fix := "aigw provider rotate " + selected.ProviderID
	switch {
	case err != nil:
		title, fix = fmt.Sprintf("%s's configuration files cannot change while the Token of provider %s cannot be observed", spec.Label, selected.ProviderID), "aigw provider show "+selected.ProviderID
	case secret.IsReadOnly(runtime.Secrets):
		fix = "Set " + secret.EnvironmentKey(selected.ProviderID) + ", then run the same command again."
	}
	return terminal.ProblemError(title, "Without the Token, AIGW writes no client file, and its settings would stay in the files that the edit drops.", "Nothing changed.", fix, err)
}

// editClient applies an edit to the client's settings in cfg. Forwarding comes
// last, because it records the upstream that the client's model reaches. An
// edit that changes nothing leaves cfg as it was.
func editClient(cfg *domain.Config, id string, options editOptions, targets []string) error {
	current := cfg.Clients[id]
	edited := current
	if options.executable != "" {
		edited.Executable = options.executable
	}
	if len(targets) > 0 {
		edited.Targets = targets
	}
	switch {
	case options.native:
		edited.RetryPolicy = nil
	case options.request.given || options.stream.given:
		policy := edited.RetryPolicy.Clone()
		if policy == nil {
			policy = &domain.RetryPolicy{}
		}
		if request := options.request.value; options.request.given {
			policy.RequestMaxRetries = &request
		}
		if stream := options.stream.value; options.stream.given {
			policy.StreamMaxRetries = &stream
		}
		edited.RetryPolicy = policy
	}
	if !reflect.DeepEqual(edited, current) {
		if cfg.Clients == nil {
			cfg.Clients = map[string]domain.ClientBinding{}
		}
		cfg.Clients[id] = edited
	}
	switch {
	case options.via != "":
		return cfg.SetForwardingEndpoint(id, strings.TrimRight(options.via, "/"))
	case options.direct && edited.ForwardingEndpoint != "":
		return cfg.SetForwardingEndpoint(id, "")
	}
	return nil
}

// describeEdit reports the client's settings in cfg as an edit's result does.
func describeEdit(cfg domain.Config, id string) editDocument {
	binding := cfg.Clients[id]
	document := editDocument{
		Client: id, Enabled: binding.Enabled, Executable: binding.Executable,
		Targets: append([]string{}, binding.Targets...), Forwarding: binding.ForwardingEndpoint,
	}
	if model, selected := cfg.Models[cfg.SelectedModel(id)]; selected {
		document.Model = model.Ref().String()
	}
	if policy := binding.RetryPolicy; policy != nil {
		document.Retries = &retriesItem{Request: policy.RequestMaxRetries, Stream: policy.StreamMaxRetries}
	}
	return document
}

// writeEdit reports an edit as JSON or for a person, after it was applied or
// as the preview that --dry-run asks for.
func writeEdit(cmd *cobra.Command, runtime invocation.Context, spec domain.ClientSpec, document editDocument, options editOptions) error {
	document.DryRun = options.dryRun
	title, success, next := "Client unchanged", "Nothing to change: "+spec.Label+" already has these settings", ""
	switch {
	case !document.Changed:
	case options.dryRun:
		title, success, next = "Client edited (preview)", "Nothing was written: configuration, credentials and client files are unchanged", invocation.ApplyCommand(cmd, cmd.Flags().Args())
	default:
		title, success = "Client edited", "Saved; the provider, its Token and the other clients are unchanged"
		if document.RestartRequired {
			next = enabledNext(spec)
		} else if document.Enabled {
			next = "aigw check"
		}
	}
	if options.jsonMode {
		document.NextAction, document.NextDetail = terminal.NextSteps(next)
		return terminal.WriteDocument(runtime.Out, editSchemaVersion, true, document)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle(title)
	r.Rows(editRows(spec, document)...)
	invocation.RenderClientFiles(r, runtime, document.ClientFiles)
	if reader := document.CredentialReader; reader != nil {
		r.Section("Credential reader")
		r.Text("Install " + reader.Path)
	}
	r.Success(success)
	r.Next(next)
	return r.Err()
}

// editRows are the rows of a client's settings after an edit.
func editRows(spec domain.ClientSpec, document editDocument) []terminal.Field {
	model, state := document.Model, "Disabled"
	if model == "" {
		model = "None selected"
	}
	if document.Enabled {
		state = "Enabled"
	}
	rows := []terminal.Field{{Label: "Client", Value: spec.Label}, {Label: "Model", Value: model}, {Label: "State", Value: state}}
	if document.Executable != "" {
		rows = append(rows, terminal.Field{Label: "Executable", Value: document.Executable})
	}
	for _, target := range document.Targets {
		rows = append(rows, terminal.Field{Label: "Configuration file", Value: target})
	}
	if document.Model == "" {
		return rows
	}
	forwarding := document.Forwarding
	if forwarding == "" {
		forwarding = "None; the client reaches its provider's endpoint"
	}
	rows = append(rows, terminal.Field{Label: "Forwarding", Value: forwarding})
	if !spec.RetryBudgets {
		return rows
	}
	if document.Retries == nil {
		return append(rows, terminal.Field{Label: "Retries", Value: "The client's own defaults"})
	}
	if document.Retries.Request != nil {
		rows = append(rows, terminal.Field{Label: "Request retries", Value: strconv.FormatUint(*document.Retries.Request, 10)})
	}
	if document.Retries.Stream != nil {
		rows = append(rows, terminal.Field{Label: "Stream retries", Value: strconv.FormatUint(*document.Retries.Stream, 10)})
	}
	return rows
}
